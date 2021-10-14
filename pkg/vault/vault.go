package vault

import (
	"github.com/idealo/aws-signing-proxy/pkg/proxy"
	"github.com/idealo/aws-signing-proxy/pkg/vault/internal"
	"net/http"
	"time"
)

type Client struct {
	restClient *internal.RestClient
	httpClient *http.Client
	baseUrl    string
	token      string
}

func NewVaultClient() *Client {
	return &Client{
		restClient: internal.NewRestClient(),
	}
}

func (c *Client) WithHttpClient(httpClient *http.Client) *Client {
	c.httpClient = httpClient
	return c
}

func (c *Client) WithBaseUrl(baseUrl string) *Client {
	c.baseUrl = baseUrl
	return c
}

func (c *Client) WithToken(token string) *Client {
	c.token = token
	return c
}

type ReadClient struct {
	path      string
	getClient *internal.GetRequest
}

func (c *Client) Read(path string) *ReadClient {
	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}
	getClient := c.restClient.
		WithBaseUrl(c.baseUrl).
		WithClient(c.httpClient).
		Get().
		WithHeader("X-Vault-Token", c.token).
		WithPath(path)

	r := &ReadClient{
		getClient: getClient,
		path:      path,
	}

	return r
}

func (r *ReadClient) Into(result interface{}) error {
	refreshedCreds := result.(*proxy.RefreshedCredentials)
	err := r.getClient.Do(result)
	refreshedCreds.ExpiresAt = time.Now().Add(time.Duration(refreshedCreds.LeaseDuration) * time.Second)
	return err
}
