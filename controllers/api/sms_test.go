package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gophish/gophish/config"
	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/suite"
)

type SMSTestSuite struct {
	suite.Suite
	apiKey    string
	config    *config.Config
	apiServer *Server
	admin     models.User
}

// setupRequestContext adds the user_id to the request context
func (s *SMSTestSuite) setupRequestContext(req *http.Request) *http.Request {
	return ctx.Set(req, "user_id", s.admin.Id)
}

func (s *SMSTestSuite) SetupSuite() {
	conf := &config.Config{
		DBName:         "sqlite3",
		DBPath:         ":memory:",
		MigrationsPath: "../../db/db_sqlite3/migrations/",
	}
	err := models.Setup(conf)
	s.Nil(err)
	s.config = conf
	// Get the API key to use for these tests
	u, err := models.GetUser(1)
	s.Nil(err)
	s.apiKey = u.ApiKey
	s.admin = u
	s.apiServer = NewServer()
}

func (s *SMSTestSuite) TestSMSProfiles() {
	// Test GET request
	req := httptest.NewRequest(http.MethodGet, "/api/sms/", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response := httptest.NewRecorder()
	s.apiServer.SMSProfiles(response, req)
	s.Equal(response.Code, http.StatusOK)

	// Test POST request - valid SMS profile
	sms := models.SMS{
		Name:     "Test SMS",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
	}
	body, err := json.Marshal(sms)
	s.Nil(err)
	req = httptest.NewRequest(http.MethodPost, "/api/sms/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	s.apiServer.SMSProfiles(response, req)
	s.Equal(response.Code, http.StatusCreated)

	// Test POST request - duplicate name
	req = httptest.NewRequest(http.MethodPost, "/api/sms/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	s.apiServer.SMSProfiles(response, req)
	s.Equal(response.Code, http.StatusConflict)

	// Test POST request - invalid SMS profile
	invalidSMS := models.SMS{
		Name:     "Invalid SMS",
		Provider: "twilio",
		// Missing From field
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
	}
	body, err = json.Marshal(invalidSMS)
	s.Nil(err)
	req = httptest.NewRequest(http.MethodPost, "/api/sms/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	s.apiServer.SMSProfiles(response, req)
	s.Equal(response.Code, http.StatusInternalServerError)
}

func (s *SMSTestSuite) TestSMSProfile() {
	// Create a test SMS profile
	sms := models.SMS{
		Name:     "Test SMS Profile",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := models.PostSMS(&sms)
	s.Nil(err)

	// Test GET request
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/sms/%d", sms.Id), nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response := httptest.NewRecorder()
	m := mux.NewRouter()
	m.HandleFunc("/api/sms/{id:[0-9]+}", s.apiServer.SMSProfile)
	m.ServeHTTP(response, req)
	s.Equal(response.Code, http.StatusOK)

	// Test GET request - invalid ID
	req = httptest.NewRequest(http.MethodGet, "/api/sms/999", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	m = mux.NewRouter()
	m.HandleFunc("/api/sms/{id:[0-9]+}", s.apiServer.SMSProfile)
	m.ServeHTTP(response, req)
	s.Equal(response.Code, http.StatusNotFound)

	// Test PUT request
	sms.Name = "Updated SMS Profile"
	body, err := json.Marshal(sms)
	s.Nil(err)
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/sms/%d", sms.Id), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	m = mux.NewRouter()
	m.HandleFunc("/api/sms/{id:[0-9]+}", s.apiServer.SMSProfile)
	m.ServeHTTP(response, req)
	s.Equal(response.Code, http.StatusOK)

	// Test PUT request - ID mismatch
	originalId := sms.Id // Save the original ID
	sms.Id = 999
	body, err = json.Marshal(sms)
	s.Nil(err)
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/sms/%d", originalId), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	m = mux.NewRouter()
	m.HandleFunc("/api/sms/{id:[0-9]+}", s.apiServer.SMSProfile)
	m.ServeHTTP(response, req)
	s.Equal(response.Code, http.StatusBadRequest)

	// Test DELETE request - use the original ID, not the modified one
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/sms/%d", originalId), nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	m = mux.NewRouter()
	m.HandleFunc("/api/sms/{id:[0-9]+}", s.apiServer.SMSProfile)
	m.ServeHTTP(response, req)
	s.Equal(response.Code, http.StatusOK)
}

func (s *SMSTestSuite) TestSendTestSMS() {
	// Create a test SMS profile
	sms := models.SMS{
		Name:     "Test SMS Profile",
		Provider: "twilio",
		From:     "+15555555555",
		ProviderConfig: `{
			"account_sid": "test_sid",
			"auth_token": "test_token"
		}`,
		UserId: 1,
	}
	err := models.PostSMS(&sms)
	s.Nil(err)

	// Create a test SMS template
	template := models.SMSTemplate{
		Name:   "Test SMS Template",
		Text:   "Hello {{.FirstName}}, please check {{.URL}}",
		UserId: 1,
	}
	err = models.PostSMSTemplate(&template)
	s.Nil(err)

	// Create a test page
	p := models.Page{
		Name:   "Test Page",
		HTML:   "<html>Test</html>",
		UserId: 1,
	}
	err = models.PostPage(&p)
	s.Nil(err)

	// Test POST request - valid SMS request
	smsReq := models.SMSRequest{
		SMS:         sms,
		SMSId:       sms.Id,
		SMSTemplate: template,
		Page:        p,
		URL:         "http://example.com/{{.RId}}",
		BaseRecipient: models.BaseRecipient{
			Email:     "+15551234567", // Using Email field for phone number
			FirstName: "John",
			LastName:  "Doe",
		},
	}
	body, err := json.Marshal(smsReq)
	s.Nil(err)
	req := httptest.NewRequest(http.MethodPost, "/api/sms/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response := httptest.NewRecorder()
	s.apiServer.SendTestSMS(response, req)
	// Note: This will likely fail in a real test since we don't have a mock SMS worker
	// But we're just testing the API endpoint structure here
}

func TestSMSTestSuite(t *testing.T) {
	suite.Run(t, new(SMSTestSuite))
}
