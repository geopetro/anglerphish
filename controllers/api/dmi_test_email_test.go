package api

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/gophish/gophish/dmi"
	"github.com/gophish/gophish/models"
)

// dialerWorker builds the dialer of a test email instead of sending it, so no network is used.
type dialerWorker struct {
	smtp    models.SMTP
	dialErr error
	called  bool
}

func (w *dialerWorker) Start()                               {}
func (w *dialerWorker) LaunchCampaign(c models.Campaign)     {}
func (w *dialerWorker) SendTestSMS(*models.SMSRequest) error { return nil }
func (w *dialerWorker) SendTestEmail(s *models.EmailRequest) error {
	w.called = true
	w.smtp = s.SMTP
	d, err := s.GetDialer()
	if err == nil && d == nil {
		err = errors.New("nil dialer")
	}
	w.dialErr = err
	return err
}

func setupDialerTest(t *testing.T) (*testContext, *dialerWorker) {
	t.Helper()
	tc := setupTest(t)
	resetEncryptionForTest(t)
	w := &dialerWorker{}
	tc.apiServer = NewServer(WithWorker(w))
	return tc, w
}

func dmiTestBody(smtp map[string]interface{}) map[string]interface{} {
	base := map[string]interface{}{
		"from_address":        "sender@example.com",
		"dmi_tenant_id":       "tenant",
		"dmi_client_id":       "client",
		"dmi_allowed_domains": "example.com",
		"dmi_folder":          "inbox",
	}
	for k, v := range smtp {
		base[k] = v
	}
	return map[string]interface{}{"email": "victim@example.com", "smtp": base}
}

func sendDMITest(t *testing.T, tc *testContext, apiKey string, smtp map[string]interface{}) (int, string) {
	t.Helper()
	w := encryptionRequest(t, tc, apiKey, http.MethodPost, "/api/util/send_test_email", dmiTestBody(smtp))
	return w.Code, w.Body.String()
}

func assertDialed(t *testing.T, code int, body string, w *dialerWorker) {
	t.Helper()
	if code != http.StatusOK || !w.called || w.dialErr != nil {
		t.Fatalf("expected dialer to be built, got %d %s (called %v, err %v)", code, body, w.called, w.dialErr)
	}
}

func TestDMITestEmailClientSecretSaved(t *testing.T) {
	tc, w := setupDialerTest(t)
	s := createDMIProfile(t, tc, "ms saved", dmi.AuthClientSecret)
	code, body := sendDMITest(t, tc, tc.apiKey, map[string]interface{}{
		"id": s.Id, "name": s.Name, "interface_type": models.InterfaceMS365DMI, "dmi_auth_method": dmi.AuthClientSecret,
	})
	assertDialed(t, code, body, w)
	if w.smtp.DMISecret != "super-secret-value" {
		t.Fatalf("expected stored secret, got %q", w.smtp.DMISecret)
	}
}

func TestDMITestEmailNameDoesNotBorrowSecret(t *testing.T) {
	tc, w := setupDialerTest(t)
	s := createDMIProfile(t, tc, "ms by name", dmi.AuthClientSecret)
	code, body := sendDMITest(t, tc, tc.apiKey, map[string]interface{}{
		"name": s.Name, "interface_type": models.InterfaceMS365DMI, "dmi_auth_method": dmi.AuthClientSecret,
	})
	if code != http.StatusBadRequest || w.called {
		t.Fatalf("an unsaved form reused a stored secret by name: %d %s", code, body)
	}
}

func TestDMITestEmailClientSecretTyped(t *testing.T) {
	tc, w := setupDialerTest(t)
	code, body := sendDMITest(t, tc, tc.apiKey, map[string]interface{}{
		"name": "unsaved", "interface_type": models.InterfaceMS365DMI, "dmi_auth_method": dmi.AuthClientSecret, "dmi_secret": "typed-secret",
	})
	assertDialed(t, code, body, w)
	if w.smtp.DMISecret != "typed-secret" {
		t.Fatalf("expected typed secret, got %q", w.smtp.DMISecret)
	}
}

func TestDMITestEmailTypedSecretOverridesSaved(t *testing.T) {
	tc, w := setupDialerTest(t)
	s := createDMIProfile(t, tc, "ms override", dmi.AuthClientSecret)
	code, body := sendDMITest(t, tc, tc.apiKey, map[string]interface{}{
		"id": s.Id, "name": s.Name, "interface_type": models.InterfaceMS365DMI, "dmi_auth_method": dmi.AuthClientSecret, "dmi_secret": "new-secret",
	})
	assertDialed(t, code, body, w)
	if w.smtp.DMISecret != "new-secret" {
		t.Fatalf("expected typed secret, got %q", w.smtp.DMISecret)
	}
	stored, err := models.GetSMTP(s.Id, 1)
	if err != nil {
		t.Fatal(err)
	}
	if stored.DMISecret != "super-secret-value" {
		t.Fatalf("test email changed the stored secret to %q", stored.DMISecret)
	}
}

func TestDMITestEmailCertificateSaved(t *testing.T) {
	tc, w := setupDialerTest(t)
	s := createDMIProfile(t, tc, "ms cert", dmi.AuthCertificate)
	stored, err := models.GetSMTP(s.Id, 1)
	if err != nil {
		t.Fatal(err)
	}
	code, body := sendDMITest(t, tc, tc.apiKey, map[string]interface{}{
		"id": s.Id, "name": s.Name, "interface_type": models.InterfaceMS365DMI, "dmi_auth_method": dmi.AuthCertificate,
		"dmi_certificate": "attacker supplied",
	})
	assertDialed(t, code, body, w)
	if w.smtp.DMISecret != stored.DMISecret || w.smtp.DMICertificate != stored.DMICertificate || !strings.Contains(w.smtp.DMISecret, "PRIVATE KEY") {
		t.Fatal("expected the stored key pair")
	}
}

func TestDMITestEmailCertificateUnsaved(t *testing.T) {
	tc, w := setupDialerTest(t)
	code, body := sendDMITest(t, tc, tc.apiKey, map[string]interface{}{
		"name": "unsaved cert", "interface_type": models.InterfaceMS365DMI, "dmi_auth_method": dmi.AuthCertificate,
	})
	if code != http.StatusBadRequest || w.called || !strings.Contains(body, "Save the profile") {
		t.Fatalf("expected 400 asking to save, got %d %s", code, body)
	}
}

func TestDMITestEmailCustomerKeyUploaded(t *testing.T) {
	tc, w := setupDialerTest(t)
	key := testServiceAccountKey(t)
	code, body := sendDMITest(t, tc, tc.apiKey, map[string]interface{}{
		"name": "gws unsaved", "interface_type": models.InterfaceGWSDMI, "dmi_auth_method": dmi.AuthCustomerKey, "dmi_secret": key,
	})
	assertDialed(t, code, body, w)
	if w.smtp.DMISecret != key {
		t.Fatal("expected the uploaded key")
	}
}

func TestDMITestEmailCustomerKeyInvalid(t *testing.T) {
	tc, w := setupDialerTest(t)
	code, body := sendDMITest(t, tc, tc.apiKey, map[string]interface{}{
		"name": "gws bad", "interface_type": models.InterfaceGWSDMI, "dmi_auth_method": dmi.AuthCustomerKey, "dmi_secret": "not json",
	})
	if code != http.StatusBadRequest || w.called || !strings.Contains(body, "client_email") {
		t.Fatalf("expected key validation error, got %d %s", code, body)
	}
}

func TestDMITestEmailOperatorKey(t *testing.T) {
	tc, w := setupDialerTest(t)
	if err := models.SaveDMIGoogleKey(testServiceAccountKey(t)); err != nil {
		t.Fatal(err)
	}
	code, body := sendDMITest(t, tc, tc.apiKey, map[string]interface{}{
		"name": "gws operator", "interface_type": models.InterfaceGWSDMI, "dmi_auth_method": dmi.AuthOperatorKey,
	})
	assertDialed(t, code, body, w)
	if w.smtp.DMISecret != "" {
		t.Fatal("operator profiles keep no secret of their own")
	}
}

func TestDMITestEmailOtherUsersProfile(t *testing.T) {
	tc, w := setupDialerTest(t)
	s := createDMIProfile(t, tc, "admin profile", dmi.AuthClientSecret)
	cert := createDMIProfile(t, tc, "admin cert", dmi.AuthCertificate)
	other := createUnpriviledgedUser(t, models.RoleUser)
	for _, smtp := range []map[string]interface{}{
		{"id": s.Id, "name": "x", "interface_type": models.InterfaceMS365DMI, "dmi_auth_method": dmi.AuthClientSecret},
		{"name": s.Name, "interface_type": models.InterfaceMS365DMI, "dmi_auth_method": dmi.AuthClientSecret},
		{"id": cert.Id, "name": cert.Name, "interface_type": models.InterfaceMS365DMI, "dmi_auth_method": dmi.AuthCertificate},
	} {
		code, body := sendDMITest(t, tc, other.ApiKey, smtp)
		if code != http.StatusBadRequest || w.called {
			t.Fatalf("another user's profile was used: %d %s", code, body)
		}
	}
}
