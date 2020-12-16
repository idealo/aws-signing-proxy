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

func NewVaultCredChain(rc ReadClient) *credentials.Credentials {
	verboseErrors := false
	return credentials.NewCredentials(&credentials.ChainProvider{
		VerboseErrors: aws.BoolValue(&verboseErrors),
		Providers: []credentials.Provider{
			NewVaultCredentialProvider(rc),
		},
	})
}

func NewVaultCredentialProvider(rc ReadClient) *VaultCredentialProvider {
	return &VaultCredentialProvider{
		client: rc,
	}
}

type VaultCredentialProvider struct {
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

func (v *VaultCredentialProvider) Retrieve() (credentials.Value, error) {
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

func (v *VaultCredentialProvider) IsExpired() bool {
	return time.Now().After(v.ExpirationDate)
}
