package proxy

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	. "github.com/idealo/aws-signing-proxy/pkg/logging"
	"go.uber.org/zap"
	"strings"
	"time"
)

type ReadClient interface {
	RefreshCredentials(result interface{}) error
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

	err := cp.client.RefreshCredentials(c)

	if err != nil {
		if strings.Contains(err.Error(), "circuit breaker is open") {
			Logger.Warn(
				"Request to authorization server failed. Circuit breaker is open.",
			)
			return credentials.Value{}, nil
		} else {
			Logger.Error("An error appeared", zap.Error(err))
			return credentials.Value{}, err
		}
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
