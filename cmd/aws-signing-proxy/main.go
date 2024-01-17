package main

import (
	"errors"
	"fmt"
	"github.com/go-co-op/gocron"
	"github.com/idealo/aws-signing-proxy/pkg/irsa"
	. "github.com/idealo/aws-signing-proxy/pkg/logging"
	"github.com/idealo/aws-signing-proxy/pkg/oidc"
	"github.com/idealo/aws-signing-proxy/pkg/proxy"
	"github.com/idealo/aws-signing-proxy/pkg/vault"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type EnvConfig struct {
	TargetUrl                   string        `required:"true" split_words:"true"`
	Port                        int           `default:"8080"`
	MgmtPort                    int           `split_words:"true" default:"8081"`
	Service                     string        `default:"es"`
	CredentialsProvider         string        `split_words:"true"`
	VaultUrl                    string        `split_words:"true"`
	VaultAuthToken              string        `split_words:"true"`
	VaultCredentialsPath        string        `split_words:"true"`
	OpenIdAuthServerUrl         string        `split_words:"true"`
	OpenIdClientId              string        `split_words:"true"`
	OpenIdClientSecret          string        `split_words:"true"`
	AsyncOpenIdCredentialsFetch bool          `split_words:"true" default:"false"`
	RoleArn                     string        `split_words:"true"`
	MetricsPath                 string        `split_words:"true" default:"/status/metrics"`
	FlushInterval               time.Duration `split_words:"true" default:"0s"`
	IdleConnTimeout             time.Duration `split_words:"true" default:"90s"`
	DialTimeout                 time.Duration `split_words:"true"  default:"30s"`
	IrsaClientId                string        `split_words:"true" default:"aws-signing-proxy"`
}

func main() {

	defer Logger.Sync()

	e := loadConfig()

	targetURL, err := url.Parse(e.TargetUrl)
	if err != nil {
		Logger.Error(err.Error())
	}

	// Region order of precedent:
	// os.Getenv("AWS_REGION") > "eu-central-1"
	region := os.Getenv("AWS_REGION")
	if len(region) == 0 {
		region = "eu-central-1"
	}

	var client proxy.ReadClient

	switch e.CredentialsProvider {
	case "irsa":
		client = newIrsaClient(e, client, region)
	case "oidc":
		client = newOidcClient(e, client, region)
	case "vault":
		client = newVaultClient(e, client)
	default:
		Logger.Warn("Using static credentials is unsafe. Please consider using some short-living credentials mechanism like IRSA, OIDC or Vault.")
	}

	signingProxy := proxy.NewSigningProxy(proxy.Config{
		Target:          targetURL,
		Region:          region,
		Service:         e.Service,
		FlushInterval:   e.FlushInterval,
		IdleConnTimeout: e.IdleConnTimeout,
		DialTimeout:     e.DialTimeout,
		AuthClient:      client,
	})

	listenString := fmt.Sprintf(":%v", e.Port)
	mgmtPortString := fmt.Sprintf(":%v", e.MgmtPort)
	Logger.Info("Listening", zap.String("port", listenString))
	Logger.Info("Forwarding traffic", zap.String("target", targetURL.String()))

	go provideMgmtEndpoint(mgmtPortString, e.MetricsPath)

	err = http.ListenAndServe(listenString, signingProxy)
	Logger.Error("Something went wrong", zap.Error(err))

}

func loadConfig() EnvConfig {
	e, err := parseEnvironmentVariables()
	if err != nil {
		Logger.Error(err.Error())
	}

	// Validate target URL
	if anyEnvVarEmpty(e.Service, e.TargetUrl) {
		Logger.Fatal("required parameter target (e.g. foo.eu-central-1.es.amazonaws.com) OR service (e.g. es) missing!")
	}
	return e
}

func parseEnvironmentVariables() (EnvConfig, error) {
	var e EnvConfig
	err := envconfig.Process("ASP", &e)

	if err != nil {
		return e, err
	}

	switch e.CredentialsProvider {

	case "oidc":
		err = assertEnvVarsAreSet([]string{"ASP_OPEN_ID_AUTH_SERVER_URL", "ASP_OPEN_ID_CLIENT_ID", "ASP_OPEN_ID_CLIENT_SECRET", "ASP_ROLE_ARN"})
		break
	case "vault":
		err = assertEnvVarsAreSet([]string{"ASP_VAULT_URL", "ASP_VAULT_PATH", "ASP_VAULT_AUTH_TOKEN"})
		break
	case "irsa":
		err = assertEnvVarsAreSet([]string{"ASP_IRSA_CLIENT_ID", "ASP_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE"})
		break
	default:
		break
	}

	return e, err
}

func assertEnvVarsAreSet(envVars []string) error {
	for _, condParam := range envVars {
		if len(strings.TrimSpace(os.Getenv(condParam))) == 0 {
			return errors.New(fmt.Sprintf("required key %s missing value", condParam))
		}
	}
	return nil
}

func newVaultClient(e EnvConfig, client proxy.ReadClient) proxy.ReadClient {
	Logger.Info("Using Credentials from Vault.", zap.String("vault-url", e.VaultUrl), zap.String("path", e.VaultCredentialsPath))
	client = vault.NewVaultClient().
		WithBaseUrl(e.VaultUrl).
		WithToken(e.VaultAuthToken).
		ReadFrom(e.VaultCredentialsPath)
	return client
}

func newIrsaClient(e EnvConfig, client proxy.ReadClient, region string) proxy.ReadClient {
	client = irsa.NewIRSAClient(region, e.IrsaClientId, e.RoleArn)
	return client
}

func newOidcClient(e EnvConfig, client proxy.ReadClient, region string) proxy.ReadClient {

	var oidcClient oidc.ReadClient
	oidcClient = *oidc.NewOIDCClient(region).
		WithAuthServerUrl(e.OpenIdAuthServerUrl).
		WithClientSecret(e.OpenIdClientSecret).
		WithClientId(e.OpenIdClientId).
		WithRoleArn(e.RoleArn).
		Build()

	if e.AsyncOpenIdCredentialsFetch == true {
		scheduler := gocron.NewScheduler(time.UTC)
		_, err := scheduler.Every(10).Seconds().StartImmediately().Do(func() {
			err := oidc.RetrieveCredentials(&oidcClient)
			if err != nil {
				Logger.Error("Something went wrong while trying to retrieve credentials", zap.Error(err))
			}
		})

		if err != nil {
			Logger.Error("Scheduled Task for retrieving refreshed OIDC credentials failed", zap.Error(err))
		}
		scheduler.StartAsync()
	}

	client = &oidcClient
	Logger.Info("Using Credentials from from OIDC with Oauth2 server", zap.String("auth-server", e.OpenIdAuthServerUrl))
	return client
}

func provideMgmtEndpoint(mgmtPort string, metricsPath string) {

	http.HandleFunc("/status/health", func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{\"status\":\"ok\"}"))
	})

	http.Handle(metricsPath, promhttp.Handler())

	zap.S().Fatal(http.ListenAndServe(mgmtPort, nil))
}

func anyEnvVarEmpty(vars ...string) bool {
	for _, envVar := range vars {
		if len(envVar) == 0 {
			return true
		}
	}
	return false
}
