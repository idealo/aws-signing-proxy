package vault

import (
	"fmt"
	"github.com/roechi/aws-signing-proxy/pkg/vault/internal"
	"net/http"
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

type WriteClient struct {
	path       string
	postClient *internal.PostRequest
}

type kubernetesAuthLoginRequest struct {
	Role string `json:"role"`
	Jwt  string `json:"jwt"`
}

func (c *Client) KubernetesAuthLogin(k8sAuthMethodName, role, jwt string) *WriteClient {
	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}

	request := &kubernetesAuthLoginRequest{
		Role: role,
		Jwt:  jwt,
	}

	postClient := c.restClient.
		WithBaseUrl(c.baseUrl).
		WithClient(c.httpClient).
		Post().
		WithPath(fmt.Sprintf("auth/%s/login", k8sAuthMethodName)).
		WithContent(request)

	r := &WriteClient{
		postClient: postClient,
		path:       k8sAuthMethodName,
	}

	return r
}

func (w *WriteClient) Into(result interface{}) error {
	return w.postClient.Do(result)
}

func (r *ReadClient) Into(result interface{}) error {
	return r.getClient.Do(result)
}
