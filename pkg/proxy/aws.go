package proxy

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/processcreds"
	"github.com/go-ini/ini"
	"log"
	"os"
	"time"
)

type ReadClient interface {
	RefreshCredentials(result interface{}) error
}

type CustomCredentialProcessProvider struct {
	credentials *credentials.Credentials
}

func (c *CustomCredentialProcessProvider) Retrieve() (credentials.Value, error) {
	creds := processcreds.NewCredentials(prepareCredentialsProcess())
	c.credentials = creds

	value, _ := creds.Get()
	return value, nil
}

func (c *CustomCredentialProcessProvider) IsExpired() bool {
	return c.credentials.IsExpired()
}

func NewCredChain(rc ReadClient) *credentials.Credentials {
	providers := []credentials.Provider{
		&credentials.EnvProvider{},                                        // query environment AWS_ACCESS_ID etc.
		&credentials.SharedCredentialsProvider{Filename: "", Profile: ""}, // use ~/.aws/credentials
		&CustomCredentialProcessProvider{},
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

func prepareCredentialsProcess() string {
	var fileName string
	if _, isSet := os.LookupEnv("AWS_SHARED_CREDENTIALS_FILE"); isSet {
		fileName = os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalln(err)
		}

		fileName = homeDir + "/.aws/credentials"
	}

	content, _ := ini.Load(fileName)
	section, _ := content.GetSection("default")
	key, _ := section.GetKey("credential_process")
	return key.Value()
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
