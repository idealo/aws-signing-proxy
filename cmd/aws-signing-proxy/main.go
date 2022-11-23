package main

import (
	"flag"
	"fmt"
	"github.com/go-co-op/gocron"
	. "github.com/idealo/aws-signing-proxy/pkg/logging"
	"github.com/idealo/aws-signing-proxy/pkg/oidc"
	"github.com/idealo/aws-signing-proxy/pkg/proxy"
	"github.com/idealo/aws-signing-proxy/pkg/vault"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

type EnvConfig struct {
	TargetUrl             string `split_words:"true"`
	Port                  int    `default:"8080"`
	MgmtPort              int    `default:"8081"`
	Service               string `default:"es"`
	CredentialsProvider   string `split_words:"true"`
	VaultUrl              string `split_words:"true"` // 'https://vaulthost'
	VaultAuthToken        string `split_words:"true"` // auth-token for accessing Vault
	VaultCredentialsPath  string `split_words:"true"` // path were aws credentials can be generated/retrieved (e.g: 'aws/creds/my-role')
	OpenIdAuthServerUrl   string `split_words:"true"`
	OpenIdClientId        string `split_words:"true"`
	OpenIdClientSecret    string `split_words:"true"`
	OpenIdFetchCredsAsync bool   `split_words:"true"`
	RoleArn               string `split_words:"true"`
}

type Flags struct {
	Target                      *string
	Port                        *int
	MgmtPort                    *int
	Service                     *string
	CredentialsProvider         *string
	VaultUrl                    *string
	VaultPath                   *string
	VaultAuthToken              *string
	OpenIdAuthServerUrl         *string
	OpenIdClientId              *string
	OpenIdClientSecret          *string
	AsyncOpenIdCredentialsFetch *bool
	RoleArn                     *string
	Region                      *string
	FlushInterval               *time.Duration
	IdleConnTimeout             *time.Duration
	DialTimeout                 *time.Duration
}

func main() {

	defer Logger.Sync()

	// Adding envconfig to allow setting key vars via ENV
	var e EnvConfig
	if err := envconfig.Process("ASP", &e); err != nil {
		Logger.Error(err.Error())
	}

	var flags = Flags{}
	parseFlags(&flags, e)

	// Validate target URL
	if anyFlagEmpty(*flags.Service, *flags.Target) {
		log.Fatal("required parameter target (e.g. foo.eu-central-1.es.amazonaws.com) OR service (e.g. es) missing!")
	}
	targetURL, err := url.Parse(*flags.Target)
	if err != nil {
		Logger.Error(err.Error())
	}

	// Region order of precedent:
	// regionFlag > os.Getenv("AWS_REGION") > "eu-central-1"
	region := *flags.Region
	if anyFlagEmpty(region) {
		region = "eu-central-1"
	}

	var client proxy.ReadClient

	if *flags.CredentialsProvider == "oidc" {
		if anyFlagEmpty(*flags.OpenIdClientId, *flags.OpenIdClientSecret, *flags.OpenIdAuthServerUrl, *flags.RoleArn) {
			log.Fatal("Missing some needed flags for OIDC! Either: openIdClientId, openIdClientSecret, openIdAuthServerUrl or roleArn")
		} else {
			client = newOidcClient(&flags, client, e)
		}
	} else if *flags.CredentialsProvider == "vault" {
		if anyFlagEmpty(*flags.VaultUrl, *flags.VaultPath, *flags.VaultAuthToken) {
			Logger.Warn("Disabling vault credentials source due to missing flags/environment variables.")
		} else {
			client = vault.NewVaultClient().
				WithBaseUrl(*flags.VaultUrl).
				WithToken(*flags.VaultAuthToken).
				ReadFrom(*flags.VaultPath)
			Logger.Info("Using Credentials from Vault.", zap.String("vault-url", e.VaultUrl), zap.String("path", e.VaultCredentialsPath))
		}
	} else {
		Logger.Warn("Using static credentials is unsafe. Please consider using some short-living credentials mechanism like Vault or OIDC.")
	}

	signingProxy := proxy.NewSigningProxy(proxy.Config{
		Target:          targetURL,
		Region:          region,
		Service:         *flags.Service,
		FlushInterval:   *flags.FlushInterval,
		IdleConnTimeout: *flags.IdleConnTimeout,
		DialTimeout:     *flags.DialTimeout,
		AuthClient:      client,
	})
	listenString := fmt.Sprintf(":%v", *flags.Port)
	mgmtPortString := fmt.Sprintf(":%v", *flags.MgmtPort)
	Logger.Info("Listening", zap.String("port", listenString))
	Logger.Info("Forwarding traffic", zap.String("target", targetURL.String()))

	go provideMgmtEndpoint(mgmtPortString)

	err = http.ListenAndServe(listenString, signingProxy)
	Logger.Error("Something went wrong", zap.Error(err))

}

func parseFlags(flags *Flags, e EnvConfig) {
	flags.Target = flag.String("target", e.TargetUrl, "target url to proxy to (e.g. foo.eu-central-1.es.amazonaws.com)")
	flags.Port = flag.Int("port", e.Port, "Listening port for proxy (e.g. 8080)")
	flags.MgmtPort = flag.Int("mgmt-port", e.MgmtPort, "Management port for proxy (e.g. 8081)")
	flags.Service = flag.String("service", e.Service, "AWS Service (e.g. es)")

	flags.CredentialsProvider = flag.String("credentials-provider", e.CredentialsProvider, "Either retrieve credentials via OpenID or Vault. Valid values are: oidc, vault")

	// Vault
	flags.VaultUrl = flag.String("vault-url", e.VaultUrl, "base url of vault (e.g. 'https://foo.vault.invalid')")
	flags.VaultPath = flag.String("vault-path", e.VaultCredentialsPath, "path for credentials (e.g. '/some-aws-engine/creds/some-aws-role')")
	flags.VaultAuthToken = flag.String("vault-token", e.VaultAuthToken, "token for authenticating with vault (NOTE: use the environment variable ASP_VAULT_AUTH_TOKEN instead)")

	// openID Connect
	flags.OpenIdAuthServerUrl = flag.String("openid-auth-server-url", e.OpenIdAuthServerUrl, "The authorization server url")
	flags.OpenIdClientId = flag.String("openid-client-id", e.OpenIdClientId, "OAuth client id")
	flags.OpenIdClientSecret = flag.String("openid-client-secret", e.OpenIdClientSecret, "Oauth client secret")
	flags.AsyncOpenIdCredentialsFetch = flag.Bool("async-open-id-creds-fetch", e.AsyncOpenIdCredentialsFetch, "Fetch AWS Credentials via OIDC asynchronously")
	flags.RoleArn = flag.String("role-arn", e.RoleArn, "AWS role ARN to assume to")

	flags.Region = flag.String("region", os.Getenv("AWS_REGION"), "AWS region for credentials (e.g. eu-central-1)")
	flags.FlushInterval = flag.Duration("flush-interval", 0, "non essential: flush interval to flush to the client while copying the response body.")
	flags.IdleConnTimeout = flag.Duration("idle-conn-timeout", 90*time.Second, "non essential: the maximum amount of time an idle (keep-alive) connection will remain idle before closing itself. zero means no limit.")
	flags.DialTimeout = flag.Duration("dial-timeout", 30*time.Second, "non essential: the maximum amount of time a dial will wait for a connect to complete.")

	flag.Parse()
}

func newOidcClient(flags *Flags, client proxy.ReadClient, e EnvConfig) proxy.ReadClient {
	var oidcClient oidc.ReadClient
	oidcClient = *oidc.NewOIDCClient(*flags.Region).
		WithAuthServerUrl(*flags.OpenIdAuthServerUrl).
		WithClientSecret(*flags.OpenIdClientSecret).
		WithClientId(*flags.OpenIdClientId).
		WithRoleArn(*flags.RoleArn).
		Build()

	if *flags.AsyncOpenIdCredentialsFetch == true {
		scheduler := gocron.NewScheduler(time.UTC)
		_, err := scheduler.Every(10).Seconds().StartImmediately().Do(func() { oidc.RetrieveCredentials(&oidcClient) })
		if err != nil {
			Logger.Error("Scheduled Task for retrieving refreshed OIDC credentials failed", zap.Error(err))
		}
		scheduler.StartAsync()
	}

	client = &oidcClient
	Logger.Info("Using Credentials from from OIDC with Oauth2 server", zap.String("auth-server", e.OpenIdAuthServerUrl))
	return client
}

func provideMgmtEndpoint(h string) {

	http.HandleFunc("/status/health", func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{\"status\":\"ok\"}"))
	})

	http.Handle("/metrics", promhttp.Handler())

	log.Fatal(http.ListenAndServe(h, nil))
}

func anyFlagEmpty(flags ...string) bool {
	for _, cliFlag := range flags {
		if len(cliFlag) == 0 {
			return true
		}
	}
	return false
}
