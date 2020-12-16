package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type RestClient struct {
	baseUrl string
	client  *http.Client
	header  http.Header
	path    string
}

func NewRestClient() *RestClient {
	return &RestClient{}
}

func (h *RestClient) WithBaseUrl(baseUrl string) *RestClient {
	h.baseUrl = baseUrl
	return h
}

func (h *RestClient) WithClient(client *http.Client) *RestClient {
	h.client = client
	return h
}

type GetRequest struct {
	httpClient *RestClient
	header     http.Header
	path       string
}

func (h *RestClient) Get() *GetRequest {
	return &GetRequest{
		httpClient: h,
		header:     map[string][]string{},
	}
}

func (g *GetRequest) WithHeader(name string, value string) *GetRequest {
	g.header.Add(name, value)
	return g
}

func (g *GetRequest) WithPath(path string) *GetRequest {
	g.path = path
	return g
}

func (g *GetRequest) Do(response interface{}) error {
	vaultTargetUrl := fmt.Sprintf("%s/v1/%s", g.httpClient.baseUrl, g.path)
	req, err := http.NewRequest(http.MethodGet, vaultTargetUrl, nil)
	if err != nil {
		return err
	}
	for name, value := range g.header {
		req.Header.Add(name, value[0])
	}
	r, err := g.httpClient.client.Do(req)
	if err != nil {
		return err
	}
	if r.StatusCode > 299 {
		return fmt.Errorf("encountered error while connecting to vault '%s'. status-code: %d", vaultTargetUrl, r.StatusCode)
	}

	return json.NewDecoder(r.Body).Decode(response)
}
