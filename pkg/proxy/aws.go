package proxy

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"strings"
	"time"
)

var logger, _ = InitLogging()

func InitLogging() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)
	return config.Build()
}

var breaker = initCircuitBreaker()

func initCircuitBreaker() *gobreaker.CircuitBreaker {
	st := gobreaker.Settings{
		Name: "auth-server-circuit-breaker",
	}
	return gobreaker.NewCircuitBreaker(st)
}

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

	_, err := breaker.Execute(
		func() (interface{}, error) {
			err := cp.client.RefreshCredentials(c)
			return nil, err
		})

	logger.Info("State", zap.String("State", breaker.State().String()))
	logger.Info("Count", zap.Int("Requests", int(breaker.Counts().Requests)))
	logger.Info("State", zap.Int("TotalFailures", int(breaker.Counts().TotalFailures)))
	logger.Info("State", zap.Int("ConsecutiveFailures", int(breaker.Counts().ConsecutiveFailures)))
	logger.Info("State", zap.Int("ConsecutiveSuccesses", int(breaker.Counts().ConsecutiveSuccesses)))
	logger.Info("State", zap.Int("TotalSuccesses", int(breaker.Counts().TotalSuccesses)))

	if err != nil {
		if strings.Contains(err.Error(), "circuit breaker is open") {
			logger.Warn(
				"Request to authorization server failed. Circuit breaker is open.",
				zap.String("name", breaker.Name()),
				zap.String("state", breaker.State().String()),
			)
			return credentials.Value{}, nil
		} else {
			logger.Error("An error appeared", zap.Error(err))
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
