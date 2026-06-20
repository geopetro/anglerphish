package mailer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	log "github.com/gophish/gophish/logger"
	"github.com/sirupsen/logrus"
	"github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

// SMSProvider is an interface that defines the operations needed for an SMS provider
type SMSProvider interface {
	Send(from string, to string, message string) error
	Close() error
}

// SMSDialer dials to an SMS provider and returns the SMSProvider
type SMSDialer interface {
	Dial() (SMSProvider, error)
}

// SMSMail is an interface that handles the common operations for SMS messages
type SMSMail interface {
	Backoff(reason error) error
	Error(err error) error
	Success() error
	Generate() (string, error)
	GetDialer() (SMSDialer, error)
	GetSMSFrom() (string, error)
	GetSMSTo() (string, error)
}

// SMSWorker is the worker that receives slices of SMS messages
// on a channel to send.
type SMSWorker struct {
	queue chan []SMSMail
}

// NewSMSWorker returns an instance of SMSWorker with the SMS queue
// initialized.
func NewSMSWorker() *SMSWorker {
	return &SMSWorker{
		queue: make(chan []SMSMail),
	}
}

// Start launches the SMS worker to begin listening on the Queue channel
// for new slices of SMSMail instances to process.
func (sw *SMSWorker) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ms := <-sw.queue:
			go func(ctx context.Context, ms []SMSMail) {
				dialer, err := ms[0].GetDialer()
				if err != nil {
					errorSMSMail(err, ms)
					return
				}
				sendSMS(ctx, dialer, ms)
			}(ctx, ms)
		}
	}
}

// Queue sends the provided SMS messages to the internal queue for processing.
func (sw *SMSWorker) Queue(ms []SMSMail) {
	sw.queue <- ms
}

// errorSMSMail is a helper to handle erroring out a slice of SMSMail instances
// in the case that an unrecoverable error occurs.
func errorSMSMail(err error, ms []SMSMail) {
	for _, m := range ms {
		m.Error(err)
	}
}

// dialSMSProvider attempts to make a connection to the provider specified by the Dialer.
// It returns MaxReconnectAttempts if the number of connection attempts has been
// exceeded.
func dialSMSProvider(ctx context.Context, dialer SMSDialer) (SMSProvider, error) {
	sendAttempt := 0
	var provider SMSProvider
	var err error
	for {
		select {
		case <-ctx.Done():
			return nil, nil
		default:
			break
		}
		provider, err = dialer.Dial()
		if err == nil {
			break
		}
		sendAttempt++
		if sendAttempt == MaxReconnectAttempts {
			err = &ErrMaxConnectAttempts{
				underlyingError: err,
			}
			break
		}
	}
	return provider, err
}

// sendSMS attempts to send the provided SMSMail instances.
// If the context is cancelled before all of the messages are sent,
// sendSMS just returns and does not modify those messages.
func sendSMS(ctx context.Context, dialer SMSDialer, ms []SMSMail) {
	provider, err := dialSMSProvider(ctx, dialer)
	if err != nil {
		log.Warn(err)
		errorSMSMail(err, ms)
		return
	}
	defer provider.Close()
	for i, m := range ms {
		select {
		case <-ctx.Done():
			return
		default:
			break
		}

		message, err := m.Generate()
		if err != nil {
			log.Warn(err)
			m.Error(err)
			continue
		}

		from, err := m.GetSMSFrom()
		if err != nil {
			m.Error(err)
			continue
		}

		to, err := m.GetSMSTo()
		if err != nil {
			m.Error(err)
			continue
		}

		// Ensure phone number is in E.164 format (starts with +)
		if to != "" && !strings.HasPrefix(to, "+") {
			to = "+" + to
			log.WithFields(logrus.Fields{
				"to": to,
			}).Info("Adding + prefix to phone number for E.164 format")
		}

		err = provider.Send(from, to, message)
		if err != nil {
			log.WithFields(logrus.Fields{
				"error": err,
				"to":    to,
			}).Warn("Error sending SMS")

			origErr := err
			provider, err = dialSMSProvider(ctx, dialer)
			if err != nil {
				errorSMSMail(err, ms[i:])
				break
			}
			m.Error(origErr)
			continue
		}

		log.WithFields(logrus.Fields{
			"from": from,
			"to":   to,
		}).Info("SMS sent")
		m.Success()
	}
}

// TwilioProvider implements the SMSProvider interface for Twilio
type TwilioProvider struct {
	AccountSID string
	AuthToken  string
	Client     *twilio.RestClient
}

// Send sends an SMS via Twilio
func (tp *TwilioProvider) Send(from string, to string, message string) error {
	params := &twilioApi.CreateMessageParams{}
	params.SetFrom(from)
	params.SetTo(to)
	params.SetBody(message)

	log.WithFields(logrus.Fields{
		"provider": "twilio",
		"from":     from,
		"to":       to,
	}).Info("Sending SMS via Twilio")

	_, err := tp.Client.Api.CreateMessage(params)
	return err
}

// Close closes the Twilio provider
func (tp *TwilioProvider) Close() error {
	return nil
}

// requiresUnicode checks if a message contains characters outside the GSM-7 character set
// and therefore requires Unicode encoding
func requiresUnicode(message string) bool {
	// GSM-7 basic character set (standard + extended)
	// This is a simplified check - if any character is outside ASCII printable range
	// or is a special character not in GSM-7, we need Unicode
	for _, r := range message {
		// Check if character is outside basic GSM-7 compatible range
		// GSM-7 supports: A-Z, a-z, 0-9, and some special characters
		// Greek, Arabic, Chinese, etc. all require Unicode
		if r > 127 {
			return true
		}
	}
	return false
}

// TwilioDialer implements the SMSDialer interface for Twilio
type TwilioDialer struct {
	AccountSID string
	AuthToken  string
}

// Dial creates a new Twilio provider
func (td *TwilioDialer) Dial() (SMSProvider, error) {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: td.AccountSID,
		Password: td.AuthToken,
	})

	return &TwilioProvider{
		AccountSID: td.AccountSID,
		AuthToken:  td.AuthToken,
		Client:     client,
	}, nil
}

// NexmoProvider implements the SMSProvider interface for Nexmo/Vonage
type NexmoProvider struct {
	APIKey    string
	APISecret string
}

// NexmoResponse represents the JSON response from Vonage SMS API
type NexmoResponse struct {
	MessageCount string `json:"message-count"`
	Messages     []struct {
		To               string `json:"to"`
		MessageID        string `json:"message-id"`
		Status           string `json:"status"`
		RemainingBalance string `json:"remaining-balance"`
		MessagePrice     string `json:"message-price"`
		Network          string `json:"network"`
		ErrorText        string `json:"error-text"`
	} `json:"messages"`
}

// Send sends an SMS via Nexmo
func (np *NexmoProvider) Send(from string, to string, message string) error {
	// Nexmo API endpoint
	apiURL := "https://rest.nexmo.com/sms/json"

	// Create form data
	formData := url.Values{}
	formData.Set("api_key", np.APIKey)
	formData.Set("api_secret", np.APISecret)
	formData.Set("from", from)
	formData.Set("to", to)
	formData.Set("text", message)

	// Check if message requires Unicode encoding (for Greek, Arabic, Chinese, etc.)
	if requiresUnicode(message) {
		formData.Set("type", "unicode")
		log.WithFields(logrus.Fields{
			"provider": "nexmo",
		}).Debug("Using Unicode encoding for SMS")
	}

	log.WithFields(logrus.Fields{
		"provider": "nexmo",
		"from":     from,
		"to":       to,
	}).Info("Sending SMS via Nexmo")

	// Make the HTTP request
	resp, err := http.Post(apiURL, "application/x-www-form-urlencoded", strings.NewReader(formData.Encode()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check for successful HTTP status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vonage API returned non-200 status code: %d", resp.StatusCode)
	}

	// Parse the JSON response to check for API-level errors
	// Vonage returns HTTP 200 even for API errors, so we must check the response body
	var nexmoResp NexmoResponse
	if err := json.NewDecoder(resp.Body).Decode(&nexmoResp); err != nil {
		return fmt.Errorf("failed to parse Vonage response: %v", err)
	}

	// Check if any messages were returned
	if len(nexmoResp.Messages) == 0 {
		return fmt.Errorf("vonage returned no message status")
	}

	// Check the status of the first message (status "0" means success)
	// Vonage API error codes:
	// 0 = Success, 1 = Throttled, 2 = Missing params, 3 = Invalid params,
	// 4 = Invalid credentials, 5 = Internal error, 6 = Invalid message,
	// 7 = Number barred, 9 = Partner quota exceeded, 15 = Invalid sender address,
	// 29 = Non-whitelisted destination
	msg := nexmoResp.Messages[0]
	if msg.Status != "0" {
		errText := msg.ErrorText
		if errText == "" {
			errText = "Unknown error"
		}
		log.WithFields(logrus.Fields{
			"provider":   "vonage",
			"status":     msg.Status,
			"error_text": errText,
			"to":         to,
		}).Warn("Vonage API returned error")
		return fmt.Errorf("vonage error (status %s): %s", msg.Status, errText)
	}

	// Log successful message details
	log.WithFields(logrus.Fields{
		"provider":   "vonage",
		"message_id": msg.MessageID,
		"network":    msg.Network,
		"to":         to,
	}).Debug("Vonage SMS accepted")

	return nil
}

// Close closes the Nexmo provider
func (np *NexmoProvider) Close() error {
	return nil
}

// NexmoDialer implements the SMSDialer interface for Nexmo
type NexmoDialer struct {
	APIKey    string
	APISecret string
}

// Dial creates a new Nexmo provider
func (nd *NexmoDialer) Dial() (SMSProvider, error) {
	return &NexmoProvider{
		APIKey:    nd.APIKey,
		APISecret: nd.APISecret,
	}, nil
}

// SMSBalanceInfo contains the balance information from an SMS provider
type SMSBalanceInfo struct {
	Balance  float64 `json:"balance"`
	Currency string  `json:"currency"`
}

// GetSMSBalance retrieves the account balance for a given SMS provider
func GetSMSBalance(provider string, config string) (*SMSBalanceInfo, error) {
	switch provider {
	case "twilio":
		var twilioConfig struct {
			AccountSID string `json:"account_sid"`
			AuthToken  string `json:"auth_token"`
		}
		err := json.Unmarshal([]byte(config), &twilioConfig)
		if err != nil {
			return nil, err
		}
		return getTwilioBalance(twilioConfig.AccountSID, twilioConfig.AuthToken)
	case "nexmo":
		var nexmoConfig struct {
			APIKey    string `json:"api_key"`
			APISecret string `json:"api_secret"`
		}
		err := json.Unmarshal([]byte(config), &nexmoConfig)
		if err != nil {
			return nil, err
		}
		return getVonageBalance(nexmoConfig.APIKey, nexmoConfig.APISecret)
	default:
		return nil, fmt.Errorf("unsupported SMS provider: %s", provider)
	}
}

// getTwilioBalance retrieves the account balance from Twilio
func getTwilioBalance(accountSID, authToken string) (*SMSBalanceInfo, error) {
	// Twilio balance API endpoint
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Balance.json", accountSID)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(accountSID, authToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twilio API returned status code: %d", resp.StatusCode)
	}

	var twilioResp struct {
		Balance  string `json:"balance"`
		Currency string `json:"currency"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&twilioResp); err != nil {
		return nil, fmt.Errorf("failed to parse Twilio balance response: %v", err)
	}

	balance := 0.0
	fmt.Sscanf(twilioResp.Balance, "%f", &balance)

	return &SMSBalanceInfo{
		Balance:  balance,
		Currency: twilioResp.Currency,
	}, nil
}

// getVonageBalance retrieves the account balance from Vonage/Nexmo
func getVonageBalance(apiKey, apiSecret string) (*SMSBalanceInfo, error) {
	// Vonage balance API endpoint
	apiURL := fmt.Sprintf("https://rest.nexmo.com/account/get-balance?api_key=%s&api_secret=%s", apiKey, apiSecret)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vonage API returned status code: %d", resp.StatusCode)
	}

	var vonageResp struct {
		Value      float64 `json:"value"`
		AutoReload bool    `json:"autoReload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&vonageResp); err != nil {
		return nil, fmt.Errorf("failed to parse Vonage balance response: %v", err)
	}

	return &SMSBalanceInfo{
		Balance:  vonageResp.Value,
		Currency: "EUR", // Vonage uses EUR by default
	}, nil
}

// GetSMSDialer returns an SMSDialer based on the provider and configuration
func GetSMSDialer(provider string, config string) (SMSDialer, error) {
	switch provider {
	case "twilio":
		var twilioConfig struct {
			AccountSID string `json:"account_sid"`
			AuthToken  string `json:"auth_token"`
		}
		err := json.Unmarshal([]byte(config), &twilioConfig)
		if err != nil {
			return nil, err
		}
		return &TwilioDialer{
			AccountSID: twilioConfig.AccountSID,
			AuthToken:  twilioConfig.AuthToken,
		}, nil
	case "nexmo":
		var nexmoConfig struct {
			APIKey    string `json:"api_key"`
			APISecret string `json:"api_secret"`
		}
		err := json.Unmarshal([]byte(config), &nexmoConfig)
		if err != nil {
			return nil, err
		}
		return &NexmoDialer{
			APIKey:    nexmoConfig.APIKey,
			APISecret: nexmoConfig.APISecret,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported SMS provider: %s", provider)
	}
}
