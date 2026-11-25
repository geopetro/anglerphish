package dmi

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"encoding/pem"
	jose "github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	defaultMicrosoftLogin = "https://login.microsoftonline.com"
	scopeGraph            = "https://graph.microsoft.com/.default"
	assertionTypeJWT      = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
)

func microsoftTokenURL(c Config) string {
	base := strings.TrimRight(c.Endpoints.MicrosoftLogin, "/")
	if base == "" {
		base = defaultMicrosoftLogin
	}
	return base + "/" + url.PathEscape(c.TenantID) + "/oauth2/v2.0/token"
}

func microsoftTokenSource(c Config) (oauth2.TokenSource, error) {
	tokenURL := microsoftTokenURL(c)
	var build func() oauth2.TokenSource
	switch c.AuthMethod {
	case AuthClientSecret:
		build = func() oauth2.TokenSource {
			ctx := context.WithValue(context.Background(), oauth2.HTTPClient, c.httpClient())
			return errorMappingSource{(&clientcredentials.Config{
				ClientID:     c.ClientID,
				ClientSecret: c.Secret,
				TokenURL:     tokenURL,
				Scopes:       []string{scopeGraph},
				AuthStyle:    oauth2.AuthStyleInParams,
			}).TokenSource(ctx)}
		}
	case AuthCertificate:
		if ts, ok := existingTokenSource(microsoftTokenKey(c)); ok {
			return ts, nil
		}
		signer, err := newAssertionSigner(c)
		if err != nil {
			return nil, err
		}
		build = func() oauth2.TokenSource {
			return &certTokenSource{c: c, tokenURL: tokenURL, signer: signer}
		}
	default:
		return nil, &Error{Err: errors.New("Auth method is not valid for Microsoft 365")}
	}
	return cachedTokenSource(microsoftTokenKey(c), build), nil
}

func microsoftTokenKey(c Config) string {
	return tokenKey(ProviderMicrosoft, c.AuthMethod, c.TenantID, c.ClientID, c.Secret, c.Certificate, microsoftTokenURL(c))
}

type assertionSigner struct {
	jose.Signer
	thumb string
}

func newAssertionSigner(c Config) (*assertionSigner, error) {
	thumb, err := CertificateThumbprint(c.Certificate)
	if err != nil {
		return nil, &Error{Err: fmt.Errorf("The certificate is invalid: %w", err)}
	}
	blk, _ := pem.Decode([]byte(c.Secret))
	if blk == nil {
		return nil, &Error{Err: errors.New("The certificate private key is invalid")}
	}
	parsed, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
	key, ok := parsed.(*rsa.PrivateKey)
	if err != nil || !ok {
		return nil, &Error{Err: errors.New("The certificate private key must be an RSA key in PKCS#8 format")}
	}
	opts := (&jose.SignerOptions{}).WithType("JWT").WithHeader("x5t", thumb)
	s, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, opts)
	if err != nil {
		return nil, &Error{Err: fmt.Errorf("could not prepare the certificate signer: %w", err)}
	}
	return &assertionSigner{Signer: s, thumb: thumb}, nil
}

type certTokenSource struct {
	c        Config
	tokenURL string
	signer   *assertionSigner
}

func (s *certTokenSource) assertion() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", &Error{Err: err}
	}
	now := time.Now()
	return jwt.Signed(s.signer).Claims(jwt.Claims{
		Audience:  jwt.Audience{s.tokenURL},
		Issuer:    s.c.ClientID,
		Subject:   s.c.ClientID,
		ID:        hex.EncodeToString(b[:]),
		NotBefore: jwt.NewNumericDate(now),
		IssuedAt:  jwt.NewNumericDate(now),
		Expiry:    jwt.NewNumericDate(now.Add(10 * time.Minute)),
	}).CompactSerialize()
}

func (s *certTokenSource) Token() (*oauth2.Token, error) {
	assertion, err := s.assertion()
	if err != nil {
		return nil, &Error{Err: fmt.Errorf("could not sign the Microsoft client assertion: %w", err)}
	}
	form := url.Values{
		"client_id":             {s.c.ClientID},
		"scope":                 {scopeGraph},
		"grant_type":            {"client_credentials"},
		"client_assertion_type": {assertionTypeJWT},
		"client_assertion":      {assertion},
	}
	req, err := http.NewRequest(http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, &Error{Err: err}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.c.httpClient().Do(req)
	if err != nil {
		return nil, &Error{Err: fmt.Errorf("could not reach Microsoft: %w", err), Temp: true}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return nil, microsoftTokenError(resp.StatusCode, body, resp.Header.Get("Retry-After"))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
		return nil, &Error{Err: errors.New("Microsoft returned an unreadable token response"), Temp: true}
	}
	return &oauth2.Token{AccessToken: out.AccessToken, TokenType: out.TokenType, Expiry: time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)}, nil
}

// errorMappingSource converts oauth2 errors from the client secret flow into *Error.
type errorMappingSource struct{ oauth2.TokenSource }

func (s errorMappingSource) Token() (*oauth2.Token, error) {
	tok, err := s.TokenSource.Token()
	if err == nil {
		return tok, nil
	}
	var re *oauth2.RetrieveError
	if errors.As(err, &re) && re.Response != nil {
		return nil, microsoftTokenError(re.Response.StatusCode, re.Body, re.Response.Header.Get("Retry-After"))
	}
	return nil, &Error{Err: fmt.Errorf("could not get a Microsoft token: %w", err), Temp: true}
}

var aadstsCode = regexp.MustCompile(`AADSTS(\d+)`)

func microsoftTokenError(status int, body []byte, retryAfter string) error {
	if status == http.StatusTooManyRequests || status >= 500 {
		return classifyHTTP(status, body, retryAfter)
	}
	var b struct {
		Desc  string `json:"error_description"`
		Codes []int  `json:"error_codes"`
	}
	json.Unmarshal(body, &b)
	if m := aadstsCode.FindStringSubmatch(b.Desc); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			b.Codes = append(b.Codes, n)
		}
	}
	for _, code := range b.Codes {
		switch code {
		case 7000215, 7000222:
			return &Error{Err: errors.New("Microsoft rejected the client secret. Check it is correct and not expired")}
		case 700027:
			return &Error{Err: errors.New("Microsoft rejected the certificate. Upload the current certificate to the app registration")}
		case 700016:
			return &Error{Err: errors.New("Microsoft could not find the app. Check the Client ID and Tenant ID")}
		case 90002:
			return &Error{Err: errors.New("Microsoft could not find the tenant. Check the Tenant ID")}
		}
	}
	if desc := strings.TrimSpace(b.Desc); desc != "" {
		return &Error{Err: fmt.Errorf("Microsoft rejected the sign-in (HTTP %d): %s", status, desc)}
	}
	return classifyHTTP(status, body, retryAfter)
}
