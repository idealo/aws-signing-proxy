package internal

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBaseUrlPathIsSet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equalf(t, "/test", r.URL.Path, "Expected target URL path did not match actual URL path.")
	}))
	defer ts.Close()

	request := NewRestClient().
		WithHttpClient(http.DefaultClient).
		WithBaseUrl(ts.URL + "/test").
		Post()

	request.Do(struct{}{})
}

func TestHeadersAreSet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equalf(t, "bar", r.Header.Get("foo"), "Expected header was not present in request.")
	}))
	defer ts.Close()

	request := NewRestClient().
		WithHttpClient(http.DefaultClient).
		WithBaseUrl(ts.URL).
		Post().
		WithHeader("foo", []string{"bar"})

	request.Do(struct{}{})
}

func TestReturnAuthzToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"idToken": "behold the mighty token"}`)
	}))
	defer ts.Close()

	request := NewRestClient().
		WithHttpClient(http.DefaultClient).
		WithBaseUrl(ts.URL).
		Post().
		WithHeader("foo", []string{"bar"})

	response, err := request.Do(struct{}{})
	assert.Nilf(t, err, "The test request returned an error.")
	assert.Equalf(t, AuthServerResponse{"behold the mighty token"}, *response,
		"Returned token did not match expectation.")
}
