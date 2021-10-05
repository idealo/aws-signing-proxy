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
	LeaseDuration int `json:"lease_duration"`
	Data          struct {
		AccessKey     string `json:"access_key"`
		SecretKey     string `json:"secret_key"`
		SecurityToken string `json:"security_token"`
	} `json:"data"`
}

func (v *CredentialProvider) Retrieve() (credentials.Value, error) {
	c := &RefreshedCredentials{}
	err := v.client.Into(c)
	if err != nil {
		log.Fatal(err)
	}

	v.ExpirationDate = time.Now().Add((time.Duration(c.LeaseDuration) - 60) * time.Second)

	return credentials.Value{
		AccessKeyID:     c.Data.AccessKey,
		SecretAccessKey: c.Data.SecretKey,
		SessionToken:    c.Data.SecurityToken,
	}, nil
}

func (v *CredentialProvider) IsExpired() bool {
	return time.Now().After(v.ExpirationDate)
}
