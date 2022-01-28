package main

import (
	"flag"
	"fmt"
	"github.com/go-co-op/gocron"
	"github.com/idealo/aws-signing-proxy/pkg/oidc"
	"github.com/idealo/aws-signing-proxy/pkg/proxy"
	"github.com/idealo/aws-signing-proxy/pkg/vault"
	"github.com/kelseyhightower/envconfig"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

type EnvConfig struct {
	TargetUrl             string `split_words:"true"`
	Port                  int    `default:"8080"`
	HealthPort            int    `default:"8081"`
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
	HealthPort                  *int
	Service                     *string
	CredentialProvider          *string
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
	// Adding envconfig to allow setting key vars via ENV
	var e EnvConfig
	if err := envconfig.Process("ASP", &e); err != nil {
		log.Fatal(err.Error())
	}

	var flags = Flags{}
	parseFlags(&flags, e)

	// Validate target URL
	if anyFlagEmpty(*flags.Service, *flags.Target) {
		log.Fatal("required parameter target (e.g. foo.eu-central-1.es.amazonaws.com) OR service (e.g. es) missing!")
	}
	targetURL, err := url.Parse(*flags.Target)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Region order of precedent:
	// regionFlag > os.Getenv("AWS_REGION") > "eu-central-1"
	region := *flags.Region
	if anyFlagEmpty(region) {
		region = "eu-central-1"
	}

	var client proxy.ReadClient

	if *flags.CredentialProvider == "oidc" {
		if anyFlagEmpty(*flags.OpenIdClientId, *flags.OpenIdClientSecret, *flags.OpenIdAuthServerUrl, *flags.RoleArn) {
			log.Fatal("Missing some needed flags for OIDC! Either: openIdClientId, openIdClientSecret, openIdAuthServerUrl or roleArn")
		} else {
			client = newOidcClient(&flags, client, e)
		}
	} else if *flags.CredentialProvider == "vault" {
		if anyFlagEmpty(*flags.VaultUrl, *flags.VaultPath, *flags.VaultAuthToken) {
			log.Println("Warning: disabling vault credentials source due to missing flags/environment variables!")
		} else {
			client = vault.NewVaultClient().
				WithBaseUrl(*flags.VaultUrl).
				WithToken(*flags.VaultAuthToken).
				ReadFrom(*flags.VaultPath)
			log.Printf("- Using Credentials from from Vault '%s' with credentialsPath '%s'\n", e.VaultUrl, e.VaultCredentialsPath)
		}
	} else {
		log.Fatal("No valid credentials provider given! Valid providers are: oidc, vault")
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
	healthPortString := fmt.Sprintf(":%v", *flags.HealthPort)
	log.Printf("Listening on %v\n", listenString)
	log.Printf("Forwarding Traffic to '%s'\n", targetURL)

	go provideHealthEndpoint(healthPortString)

	log.Fatal(http.ListenAndServe(listenString, signingProxy))

}

func parseFlags(flags *Flags, e EnvConfig) {
	flags.Target = flag.String("target", e.TargetUrl, "target url to proxy to (e.g. foo.eu-central-1.es.amazonaws.com)")
	flags.Port = flag.Int("port", e.Port, "listening port for proxy (e.g. 3000)")
	flags.HealthPort = flag.Int("healthPort", e.HealthPort, "Health port for proxy (e.g. 8081)")
	flags.Service = flag.String("service", e.Service, "AWS Service (e.g. es)")

	flags.CredentialProvider = flag.String("credentialsProvider", e.CredentialsProvider, "Either retrieve credentials via OpenID or Vault. Valid values are: oidc, vault")

	// Vault
	flags.VaultUrl = flag.String("vaultUrl", e.VaultUrl, "base url of vault (e.g. 'https://foo.vault.invalid')")
	flags.VaultPath = flag.String("vaultPath", e.VaultCredentialsPath, "path for credentials (e.g. '/some-aws-engine/creds/some-aws-role')")
	flags.VaultAuthToken = flag.String("vaultToken", e.VaultAuthToken, "token for authenticating with vault (NOTE: use the environment variable ASP_VAULT_AUTH_TOKEN instead)")

	// openID Connect
	flags.OpenIdAuthServerUrl = flag.String("openIdAuthServerUrl", e.OpenIdAuthServerUrl, "The authorization server url")
	flags.OpenIdClientId = flag.String("openIdClientId", e.OpenIdClientId, "OAuth client id")
	flags.OpenIdClientSecret = flag.String("openIdClientSecret", e.OpenIdClientSecret, "Oauth client secret")
	flags.AsyncOpenIdCredentialsFetch = flag.Bool("open-id-fetch-creds-async", e.OpenIdFetchCredsAsync, "Fetch AWS Credentials via OIDC asynchronously")
	flags.RoleArn = flag.String("roleArn", e.RoleArn, "AWS role ARN to assume to")

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
			log.Fatalf("Scheduled Task for retrieving refreshed OIDC credentials failed! %s", err)
		}
		scheduler.StartAsync()
	}

	client = &oidcClient
	log.Printf("- Using Credentials from from OIDC with Oauth2 Server '%s'\n", e.OpenIdAuthServerUrl)
	return client
}

func provideHealthEndpoint(h string) {
	http.HandleFunc("/status/health", func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{\"status\":\"ok\"}"))
	})
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
