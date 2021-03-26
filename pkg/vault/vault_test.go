package vault

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"testing"
)

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func newTestClient(fn roundTripFunc) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(fn),
	}
}

type kubernetesAuthLoginResponse struct {
	Auth kubernetesAuthLoginAuthResponse `json:"auth"`
}

type kubernetesAuthLoginAuthResponse struct {
	ClientToken   string `json:"client_token"`
	LeaseDuration int    `json:"lease_duration"`
}

func TestListRoles(t *testing.T) {
	url := "http://foo.invalid"

	client := newTestClient(func(req *http.Request) *http.Response {
		expectedURL := fmt.Sprintf("%s/v1/auth/kubernetes/login", url)
		actualURL := req.URL.String()
		body, err := ioutil.ReadAll(req.Body)
		assert.Nil(t, err)
		bodyStr := string(body)
		assert.Equal(t, expectedURL, actualURL)
		assert.Equal(t, "{\"role\":\"dev-role\",\"jwt\":\"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9\"}", bodyStr)
		return &http.Response{
			StatusCode: 200,
			Body: ioutil.NopCloser(bytes.NewBufferString(`
			{
			  "auth": {
				"client_token": "62b858f9-529c-6b26-e0b8-0457b6aacdb4",
				"accessor": "afa306d0-be3d-c8d2-b0d7-2676e1c0d9b4",
				"policies": ["default"],
				"metadata": {
				  "role": "test",
				  "service_account_name": "vault-auth",
				  "service_account_namespace": "default",
				  "service_account_secret_name": "vault-auth-token-pd21c",
				  "service_account_uid": "aa9aa8ff-98d0-11e7-9bb7-0800276d99bf"
				},
				"lease_duration": 2764800,
				"renewable": true
			  }
			}`)),
			// Must be set to non-nil value or it panics
			Header: make(http.Header),
		}
	})

	response := &kubernetesAuthLoginResponse{}
	vaultClient := NewVaultClient().
		WithHttpClient(client).
		WithBaseUrl("http://foo.invalid").
		KubernetesAuthLogin("kubernetes", "dev-role", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")
	err := vaultClient.Into(response)

	expectedResponse := &kubernetesAuthLoginResponse{
		Auth: kubernetesAuthLoginAuthResponse{
			ClientToken:   "62b858f9-529c-6b26-e0b8-0457b6aacdb4",
			LeaseDuration: 2764800,
		},
	}
	assert.Nil(t, err)
	assert.Equal(t, expectedResponse, response)
}
