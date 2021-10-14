package proxy

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"log"
	"time"
)

type ReadClient interface {
	Into(result interface{}) error
}

func NewCredChain(rc ReadClient) *credentials.Credentials {
	providers := []credentials.Provider{
		&credentials.EnvProvider{},                                        // query environment AWS_ACCESS_ID etc.
		&credentials.SharedCredentialsProvider{Filename: "", Profile: ""}, // use ~/.aws/credentials
	}
	if rc != nil {
		providers = append(providers, NewCredentialProvider(rc))
	}
	verboseErrors := false

	return credentials.NewCredentials(&credentials.ChainProvider{
		VerboseErrors: aws.BoolValue(&verboseErrors),
		Providers:     providers,
	})
}

func NewCredentialProvider(rc ReadClient) *CredentialProvider {
	return &CredentialProvider{
		client: rc,
	}
}

type CredentialProvider struct {
	client          ReadClient
	ExpirationDate  time.Time
	AccessKey       string
	SecretAccessKey string
	SessionToken    string
}

type RefreshedCredentials struct {
	ExpiresAt     time.Time `json:"expires_at"`
	LeaseDuration int       `json:"lease_duration"`
	Data          struct {
		AccessKey     string `json:"access_key"`
		SecretKey     string `json:"secret_key"`
		SecurityToken string `json:"security_token"`
	} `json:"data"`
}

func (cp *CredentialProvider) Retrieve() (credentials.Value, error) {
	c := &RefreshedCredentials{}
	err := cp.client.Into(c)
	if err != nil {
		log.Fatal(err)
	}

	cp.ExpirationDate = c.ExpiresAt

	return credentials.Value{
		AccessKeyID:     c.Data.AccessKey,
		SecretAccessKey: c.Data.SecretKey,
		SessionToken:    c.Data.SecurityToken,
	}, nil
}

func (cp *CredentialProvider) IsExpired() bool {
	return time.Now().After(cp.ExpirationDate.Add(-time.Second * 60))
}
