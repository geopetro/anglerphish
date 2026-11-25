package dmi

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// dropAfterRead reads the whole request, then closes the connection without answering.
func dropAfterRead(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Fatal(err)
		}
		conn.Close()
	}))
}

func transportErr(t *testing.T, err error) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("want *Error, got %T %v", err, err)
	}
	return e
}

func TestSendTrackedNotSentIsRetryable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+"/messages", strings.NewReader("{}"))
	_, err = sendTracked(http.DefaultClient, req, "Test")
	if e := transportErr(t, err); !e.Retryable() {
		t.Fatalf("connection refused must be retryable: %v", e)
	}
}

func TestSendTrackedDroppedAfterPostIsPermanent(t *testing.T) {
	srv := dropAfterRead(t)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/messages", strings.NewReader(`{"subject":"x"}`))
	_, err := sendTracked(srv.Client(), req, "Test")
	e := transportErr(t, err)
	if e.Retryable() {
		t.Fatalf("a POST that reached the server must not be retried: %v", e)
	}
	if !strings.Contains(e.Error(), "may already have been delivered") {
		t.Fatalf("message should warn about delivery: %v", e)
	}
}

func TestSendTrackedDroppedAfterGetIsRetryable(t *testing.T) {
	srv := dropAfterRead(t)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/mailFolders", nil)
	_, err := sendTracked(srv.Client(), req, "Test")
	if e := transportErr(t, err); !e.Retryable() {
		t.Fatalf("a lookup is safe to retry: %v", e)
	}
}
