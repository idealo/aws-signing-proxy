package vault

import (
	"encoding/json"
	"github.com/idealo/aws-signing-proxy/pkg/proxy"
	"github.com/idealo/aws-signing-proxy/pkg/vault/internal"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRefreshCredentials(t *testing.T) {

	vaultSecretpath := "/my-vault-secret/aws-secret"

	mockServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/" + vaultSecretpath:
			response := proxy.RefreshedCredentials{
				ExpiresAt:     time.Time{},
				LeaseDuration: 0,
				Data: struct {
					AccessKey     string `json:"access_key"`
					SecretKey     string `json:"secret_key"`
					SecurityToken string `json:"security_token"`
				}{
					AccessKey:     "fooAccessKey",
					SecretKey:     "barSecretKey",
					SecurityToken: "foobarSecurityToken",
				},
			}
			bytes, _ := json.Marshal(response)
			_, _ = w.Write(bytes)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))

	mockServer.Start()

	c := Client{restClient: internal.NewRestClient()}

	getClient := c.restClient.
		WithBaseUrl(mockServer.URL).
		WithClient(http.DefaultClient).
		Get().
		WithHeader("X-Vault-Token", c.token).
		WithPath(vaultSecretpath)

	client := ReadClient{
		path:      "foo",
		getClient: getClient,
	}

	rc := &proxy.RefreshedCredentials{}

	err := client.RefreshCredentials(rc)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "fooAccessKey", rc.Data.AccessKey)
	assert.Equal(t, "barSecretKey", rc.Data.SecretKey)
	assert.Equal(t, "foobarSecurityToken", rc.Data.SecurityToken)

}
