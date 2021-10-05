package internal

import (
	"bytes"
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

func (h *RestClient) WithHttpClient(client *http.Client) *RestClient {
	h.client = client
	return h
}

func (p *PostRequest) WithHeader(key string, value []string) *PostRequest {
	p.header[key] = value
	return p
}

type PostRequest struct {
	httpClient *RestClient
	header     http.Header
	path       string
	body       PostBody
}

type PostBody struct {
	Identity string `json:"identity"`
	Secret   string `json:"secret"`
}

func (h *RestClient) Post() *PostRequest {
	return &PostRequest{
		httpClient: h,
		header:     map[string][]string{},
	}
}

func (p *PostRequest) WithClientCredentials(id string, secret string) *PostRequest {
	p.body.Identity = id
	p.body.Secret = secret
	return p
}

type AuthServerResponse struct {
	IdToken string `json:"idToken"`
}

func (p *PostRequest) Do(response interface{}) (*AuthServerResponse, error) {
	authServerUrl := p.httpClient.baseUrl
	body, err := json.MarshalIndent(p.body, "", "")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, authServerUrl, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	for name, value := range p.header {
		req.Header.Add(name, value[0])
	}
	r, err := p.httpClient.client.Do(req)
	if err != nil {
		return nil, err
	}
	if r.StatusCode > 299 {
		return nil, fmt.Errorf("encountered error while connecting to auth server '%s'. status-code: %d", authServerUrl, r.StatusCode)
	}

	var authServerResponse AuthServerResponse
	err = json.NewDecoder(r.Body).Decode(&authServerResponse)
	return &authServerResponse, nil
}
