package oidc

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/idealo/aws-signing-proxy/pkg/oidc/internal"
	"github.com/idealo/aws-signing-proxy/pkg/proxy"
	"net/http"
	"time"
)

type ReadClient struct {
	restClient    *internal.RestClient
	httpClient    *http.Client
	postClient    *internal.PostRequest
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

func (c *ReadClient) Read() *ReadClient {
	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}
	postClient := internal.NewRestClient().
		WithBaseUrl(c.authServerUrl).
		WithHttpClient(c.httpClient).
		Post().
		WithClientCredentials(c.clientId, c.clientSecret).
		WithHeader("Content-Type", []string{"application/json"})

	c.postClient = postClient

	return c
}

func (r *ReadClient) retrieveShortLivingCreds(roleArn string, webToken string, roleSessionName string) *sts.Credentials {
	identity, _ := r.stsClient.AssumeRoleWithWebIdentity(&sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          &roleArn,
		RoleSessionName:  &roleSessionName,
		WebIdentityToken: &webToken,
	})

	return identity.Credentials
}

func InitClient(region string) stsiface.STSAPI {
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.AnonymousCredentials},
	))

	return sts.New(sess, aws.NewConfig().WithRegion(region))
}

func (r *ReadClient) Into(result interface{}) error {
	refCreds := result.(*proxy.RefreshedCredentials)

	res, err := r.postClient.Do(internal.AuthServerResponse{})

	stsCreds := r.retrieveShortLivingCreds(r.roleArn, res.IdToken, r.clientId)

	refCreds.LeaseDuration = int(stsCreds.Expiration.Sub(time.Now()).Seconds())
	refCreds.Data.AccessKey = *stsCreds.AccessKeyId
	refCreds.Data.SecretKey = *stsCreds.SecretAccessKey
	refCreds.Data.SecurityToken = *stsCreds.SessionToken

	return err
}
