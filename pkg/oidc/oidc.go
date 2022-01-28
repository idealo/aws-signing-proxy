package oidc

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/idealo/aws-signing-proxy/pkg/oidc/internal"
	"github.com/idealo/aws-signing-proxy/pkg/proxy"
	"log"
	"net/http"
	"time"
)

var cachedCredentials *sts.Credentials

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

func (c *ReadClient) retrieveShortLivedCreds(roleArn string, webToken string, roleSessionName string) *sts.Credentials {
	identity, err := c.stsClient.AssumeRoleWithWebIdentity(&sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          &roleArn,
		RoleSessionName:  &roleSessionName,
		WebIdentityToken: &webToken,
	})

	if err != nil {
		log.Printf("Something went wrong with the STS Client: %s\n", err)
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

func (c *ReadClient) Into(result interface{}) error {
	refreshedCreds := result.(*proxy.RefreshedCredentials)

	stsCreds := cachedCredentials

	refreshedCreds.LeaseDuration = int(stsCreds.Expiration.Sub(time.Now()).Seconds())
	refreshedCreds.Data.AccessKey = *stsCreds.AccessKeyId
	refreshedCreds.Data.SecretKey = *stsCreds.SecretAccessKey
	refreshedCreds.Data.SecurityToken = *stsCreds.SessionToken

	return nil
}

func RetrieveCredentialsAsync(c *ReadClient) {
	for true {
		if cachedCredentials == nil || isExpired(cachedCredentials.Expiration) {
			res, err := c.postClient.Do()
			if err != nil {
				log.Fatal(err)
			}
			cachedCredentials = c.retrieveShortLivedCreds(c.roleArn, res.IdToken, c.clientId)
			log.Println("Refreshed short lived credentials.")
		} else {
			time.Sleep(30 * time.Second)
		}
	}
}

func isExpired(expiration *time.Time) bool {
	return time.Now().Add(time.Minute * 5).After(*expiration)
}
