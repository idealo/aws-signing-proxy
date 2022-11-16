package oidc

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/idealo/aws-signing-proxy/pkg/circuitbreaker"
	. "github.com/idealo/aws-signing-proxy/pkg/logging"
	"github.com/idealo/aws-signing-proxy/pkg/oidc/internal"
	"github.com/idealo/aws-signing-proxy/pkg/proxy"
	"go.uber.org/zap"
	"net/http"
	"time"
)

var cachedCredentials *sts.Credentials

type ReadClient struct {
	restClient    *internal.RestClient
	httpClient    *http.Client
	postRequest   *internal.PostRequest
	stsClient     stsiface.STSAPI
	authServerUrl string
	clientId      string
	clientSecret  string
	roleArn       string
}

func NewOIDCClient(region string) *ReadClient {
	return &ReadClient{
		stsClient: InitClient(region),
	}
}

func (c *ReadClient) WithHttpClient(httpClient *http.Client) *ReadClient {
	c.httpClient = httpClient
	return c
}

func (c *ReadClient) WithAuthServerUrl(authServerUrl string) *ReadClient {
	c.authServerUrl = authServerUrl
	return c
}

func (c *ReadClient) WithClientSecret(clientSecret string) *ReadClient {
	c.clientSecret = clientSecret
	return c
}

func (c *ReadClient) WithClientId(clientId string) *ReadClient {
	c.clientId = clientId
	return c
}

func (c *ReadClient) WithRoleArn(roleArn string) *ReadClient {
	c.roleArn = roleArn
	return c
}

func (c *ReadClient) Build() *ReadClient {
	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}
	postRequest := internal.NewRestClient().
		WithBaseUrl(c.authServerUrl).
		WithHttpClient(c.httpClient).
		Post().
		WithClientCredentials(c.clientId, c.clientSecret).
		WithHeader("Content-Type", []string{"application/json"})

	c.postRequest = postRequest

	return c
}

func (c *ReadClient) retrieveShortLivingCredentialsFromAwsSts(roleArn string, webToken string, roleSessionName string) *sts.Credentials {
	identity, err := c.stsClient.AssumeRoleWithWebIdentity(&sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          &roleArn,
		RoleSessionName:  &roleSessionName,
		WebIdentityToken: &webToken,
	})

	if err != nil {
		Logger.Error("Something went wrong with the STS Client", zap.Error(err))
	}

	return identity.Credentials
}

func InitClient(region string) stsiface.STSAPI {
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.AnonymousCredentials},
	))

	return sts.New(sess, aws.NewConfig().WithRegion(region))
}

func (c *ReadClient) RefreshCredentials(result interface{}) error {
	refreshedCredentials := result.(*proxy.RefreshedCredentials)

	err := RetrieveCredentials(c)
	if err != nil {
		return err
	}
	stsCredentials := cachedCredentials

	refreshedCredentials.ExpiresAt = *stsCredentials.Expiration
	refreshedCredentials.Data.AccessKey = *stsCredentials.AccessKeyId
	refreshedCredentials.Data.SecretKey = *stsCredentials.SecretAccessKey
	refreshedCredentials.Data.SecurityToken = *stsCredentials.SessionToken

	return nil
}

var breaker = circuitbreaker.NewCircuitBreaker("oidc")

func RetrieveCredentials(c *ReadClient) error {
	if cachedCredentials == nil || isExpired(cachedCredentials.Expiration) {

		response, err := breaker.Execute(func() (interface{}, error) {
			return c.postRequest.Do()
		})

		if err != nil {
			return err
		}

		cachedCredentials = c.retrieveShortLivingCredentialsFromAwsSts(c.roleArn, response.(*internal.AuthServerResponse).IdToken, c.clientId)
		Logger.Info("Refreshed short living credentials.")
	}
	return nil
}

func isExpired(expiration *time.Time) bool {
	// subtract 5 minutes from the actual expiration to retrieve every 55 minutes new credentials
	return time.Now().After(expiration.Add(-time.Minute * 5))
}
