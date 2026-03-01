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

type SMSTemplateTestSuite struct {
	suite.Suite
	apiKey    string
	config    *config.Config
	apiServer *Server
	admin     models.User
}

// setupRequestContext adds the user_id to the request context
func (s *SMSTemplateTestSuite) setupRequestContext(req *http.Request) *http.Request {
	return ctx.Set(req, "user_id", s.admin.Id)
}

func (s *SMSTemplateTestSuite) SetupSuite() {
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

func (s *SMSTemplateTestSuite) TestSMSTemplates() {
	// Test GET request
	req := httptest.NewRequest(http.MethodGet, "/api/sms_templates/", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response := httptest.NewRecorder()
	s.apiServer.SMSTemplates(response, req)
	s.Equal(response.Code, http.StatusOK)

	// Test POST request - valid SMS template
	template := models.SMSTemplate{
		Name: "Test SMS Template",
		Text: "This is a test SMS template with {{.FirstName}} and {{.URL}}",
	}
	body, err := json.Marshal(template)
	s.Nil(err)
	req = httptest.NewRequest(http.MethodPost, "/api/sms_templates/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	s.apiServer.SMSTemplates(response, req)
	s.Equal(response.Code, http.StatusCreated)

	// Test POST request - duplicate name
	req = httptest.NewRequest(http.MethodPost, "/api/sms_templates/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	s.apiServer.SMSTemplates(response, req)
	s.Equal(response.Code, http.StatusConflict)

	// Test POST request - invalid SMS template
	invalidTemplate := models.SMSTemplate{
		Name: "Invalid SMS Template",
		// Missing Text field
	}
	body, err = json.Marshal(invalidTemplate)
	s.Nil(err)
	req = httptest.NewRequest(http.MethodPost, "/api/sms_templates/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	s.apiServer.SMSTemplates(response, req)
	s.Equal(response.Code, http.StatusBadRequest)
}

func (s *SMSTemplateTestSuite) TestSMSTemplate() {
	// Create a test SMS template
	template := models.SMSTemplate{
		Name:   "Test SMS Template",
		Text:   "This is a test SMS template with {{.FirstName}} and {{.URL}}",
		UserId: 1,
	}
	err := models.PostSMSTemplate(&template)
	s.Nil(err)

	// Test GET request
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/sms_templates/%d", template.Id), nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response := httptest.NewRecorder()
	m := mux.NewRouter()
	m.HandleFunc("/api/sms_templates/{id:[0-9]+}", s.apiServer.SMSTemplate)
	m.ServeHTTP(response, req)
	s.Equal(response.Code, http.StatusOK)

	// Test GET request - invalid ID
	req = httptest.NewRequest(http.MethodGet, "/api/sms_templates/999", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	m = mux.NewRouter()
	m.HandleFunc("/api/sms_templates/{id:[0-9]+}", s.apiServer.SMSTemplate)
	m.ServeHTTP(response, req)
	s.Equal(response.Code, http.StatusNotFound)

	// Test PUT request
	template.Text = "This is an updated SMS template"
	body, err := json.Marshal(template)
	s.Nil(err)
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/sms_templates/%d", template.Id), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	m = mux.NewRouter()
	m.HandleFunc("/api/sms_templates/{id:[0-9]+}", s.apiServer.SMSTemplate)
	m.ServeHTTP(response, req)
	s.Equal(response.Code, http.StatusOK)

	// Test PUT request - ID mismatch
	template.Id = 999
	body, err = json.Marshal(template)
	s.Nil(err)
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/sms_templates/%d", 1), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	m = mux.NewRouter()
	m.HandleFunc("/api/sms_templates/{id:[0-9]+}", s.apiServer.SMSTemplate)
	m.ServeHTTP(response, req)
	s.Equal(response.Code, http.StatusBadRequest)

	// Test PUT request - invalid template
	template.Id = 1
	template.Text = "" // Empty text field
	body, err = json.Marshal(template)
	s.Nil(err)
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/sms_templates/%d", template.Id), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	m = mux.NewRouter()
	m.HandleFunc("/api/sms_templates/{id:[0-9]+}", s.apiServer.SMSTemplate)
	m.ServeHTTP(response, req)
	s.Equal(response.Code, http.StatusBadRequest)

	// Test DELETE request
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/sms_templates/%d", template.Id), nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req = s.setupRequestContext(req)
	response = httptest.NewRecorder()
	m = mux.NewRouter()
	m.HandleFunc("/api/sms_templates/{id:[0-9]+}", s.apiServer.SMSTemplate)
	m.ServeHTTP(response, req)
	s.Equal(response.Code, http.StatusOK)
}

func TestSMSTemplateTestSuite(t *testing.T) {
	suite.Run(t, new(SMSTemplateTestSuite))
}
