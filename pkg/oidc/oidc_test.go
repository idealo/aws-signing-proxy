package oidc

import (
	"encoding/json"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestRetrieveCredentialsAheadOfTime(t *testing.T) {

	type MockOauthServerResponse struct {
		IdToken   string `json:"id_token"`
		ExpiresIn int    `json:"expires_in"`
	}

	mockServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := MockOauthServerResponse{
			IdToken:   "shortLivedIdToken",
			ExpiresIn: 3599,
		}
		bytes, _ := json.Marshal(response)
		_, _ = w.Write(bytes)
	}))

	mockServer.Start()

	readClient := ReadClient{
		restClient:    nil,
		httpClient:    nil,
		postRequest:   nil,
		stsClient:     &mockStsClient{},
		authServerUrl: mockServer.URL,
		clientId:      "client_id",
		clientSecret:  "client_secret",
		roleArn:       "role_arn",
	}
	client := readClient.Build()

	RetrieveCredentialsAheadOfTime(client)

	var accessKeyId, secretAccessKey, sessionToken string
	accessKeyId = "accessKeyId"
	secretAccessKey = "secretAccessKey"
	sessionToken = "sessionToken"
	var expiration = time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(60) * time.Minute)

	want := &sts.Credentials{
		AccessKeyId:     &accessKeyId,
		Expiration:      &expiration,
		SecretAccessKey: &secretAccessKey,
		SessionToken:    &sessionToken,
	}

	got := cachedCredentials

	if !reflect.DeepEqual(cachedCredentials, want) {
		t.Errorf("RetrieveCredentialsAheadOfTime() = %v, want %v", got, want)
	}

	defer mockServer.Close()
}

func TestRetrieveShortLivingCredentials(t *testing.T) {
	client := ReadClient{
		stsClient: &mockStsClient{},
	}

	var accessKeyId, secretAccessKey, sessionToken string
	accessKeyId = "accessKeyId"
	secretAccessKey = "secretAccessKey"
	sessionToken = "sessionToken"
	var expiration = time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(60) * time.Minute)

	want := &sts.Credentials{
		AccessKeyId:     &accessKeyId,
		Expiration:      &expiration,
		SecretAccessKey: &secretAccessKey,
		SessionToken:    &sessionToken,
	}
	got := client.retrieveShortLivingCredentialsFromAwsSts("foo", "bar", "session")

	if !reflect.DeepEqual(got, want) {
		t.Errorf("retrieveShortLivingCredentialsFromAwsSts() = %v, want %v", got, want)
	}
}

type mockStsClient struct {
	stsiface.STSAPI
}

func (*mockStsClient) AssumeRoleWithWebIdentity(*sts.AssumeRoleWithWebIdentityInput) (*sts.AssumeRoleWithWebIdentityOutput, error) {
	var accessKeyId, secretAccessKey, sessionToken string
	accessKeyId = "accessKeyId"
	secretAccessKey = "secretAccessKey"
	sessionToken = "sessionToken"
	var expiration = time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(60) * time.Minute)

	return &sts.AssumeRoleWithWebIdentityOutput{
		Credentials: &sts.Credentials{
			AccessKeyId:     &accessKeyId,
			Expiration:      &expiration,
			SecretAccessKey: &secretAccessKey,
			SessionToken:    &sessionToken,
		},
	}, nil
}
