package dmi

import (
	"fmt"
	"net/http"
	"net/http/httptrace"
)

// sendTracked sends req and classifies a transport failure by whether the request reached the provider.
// A request that was fully written may already have taken effect, so only lookups are retried then.
func sendTracked(client *http.Client, req *http.Request, service string) (*http.Response, error) {
	wrote := false
	trace := &httptrace.ClientTrace{
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err == nil {
				wrote = true
			}
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	resp, err := client.Do(req)
	if err == nil {
		return resp, nil
	}
	if !wrote || req.Method == http.MethodGet {
		return nil, &Error{Err: fmt.Errorf("could not reach %s: %w", service, err), Temp: true}
	}
	return nil, &Error{Err: fmt.Errorf("lost the connection to %s after sending the request, so the message may already have been delivered. Check the mailbox before resending: %w", service, err)}
}
