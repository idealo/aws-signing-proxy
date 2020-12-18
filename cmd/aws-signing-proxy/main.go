package main

import (
	"flag"
	"fmt"
	"github.com/roechi/aws-signing-proxy/pkg/proxy"
	"github.com/roechi/aws-signing-proxy/pkg/vault"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type EnvConfig struct {
	TargetUrl            string `split_words:"true"`
	Port                 int    `default:"8080"`
	Service              string `default:"es"`
	VaultUrl             string `split_words:"true"` // 'https://vaulthost'
	VaultAuthToken       string `split_words:"true"` // auth-token for accessing Vault
	VaultCredentialsPath string `split_words:"true"` // path were aws credentials can be generated/retrieved (e.g: 'aws/creds/my-role')
}

func main() {
	// Adding envconfig to allow setting key vars via ENV
	var e EnvConfig
	if err := envconfig.Process("ASP", &e); err != nil {
		log.Fatal(err.Error())
	}

	var targetFlag = flag.String("target", e.TargetUrl, "target url to signingProxy to")
	var portFlag = flag.Int("port", e.Port, "listening port for signingProxy")
	var serviceFlag = flag.String("service", e.Service, "AWS Service.")
	var vaultUrlFlag = flag.String("vaultUrl", e.VaultUrl, "base url of vault e.g. 'https://foo.vault.invalid'")
	var vaultPathFlag = flag.String("vaultPath", e.VaultCredentialsPath, "path for credentials e.g. '/some-aws-engine/creds/some-aws-role'")
	var vaultAuthTokenFlag = flag.String("vaultToken", e.VaultAuthToken, "token for authenticating with vault (NOTE: use the env variable ASP_VAULT_AUTH_TOKEN instead)")

	var regionFlag = flag.String("region", os.Getenv("AWS_REGION"), "AWS region for credentials")
	var flushInterval = flag.Duration("flush-interval", 0, "Flush interval to flush to the client while copying the response body.")
	var idleConnTimeout = flag.Duration("idle-conn-timeout", 90*time.Second, "the maximum amount of time an idle (keep-alive) connection will remain idle before closing itself. Zero means no limit.")
	var dialTimeout = flag.Duration("dial-timeout", 30*time.Second, "The maximum amount of time a dial will wait for a connect to complete.")

	flag.Parse()

	// Validate target URL
	if len(*targetFlag) == 0 {
		log.Fatal("Requires target URL to signingProxy to. Please use the -target flag")
	}
	targetURL, err := url.Parse(*targetFlag)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Validate vault base Url
	if len(*vaultUrlFlag) == 0 {
		log.Fatal("requires base URL to vault. please use the -url flag or the env variable")
	}
	_, err = url.Parse(*vaultUrlFlag)
	if err != nil {
		log.Fatal(err)
	}

	if len(*vaultPathFlag) == 0 {
		log.Fatal("requires path for vault secret. please use the -path flag or the env variable")
	}

	if len(*vaultAuthTokenFlag) == 0 {
		log.Fatal("requires auth token for vault. please use the -token flag or the env variable")
	}

	// Region order of precident:
	// regionFlag > os.Getenv("AWS_REGION") > "eu-central-1"
	region := *regionFlag
	if len(region) == 0 {
		region = "eu-central-1"
	}

	vaultClient := vault.NewVaultClient().
		WithBaseUrl(*vaultUrlFlag).
		WithToken(*vaultAuthTokenFlag).
		Read(*vaultPathFlag)

	signingProxy := proxy.NewSigningProxy(proxy.Config{
		Target:          targetURL,
		Region:          region,
		Service:         *serviceFlag,
		FlushInterval:   *flushInterval,
		IdleConnTimeout: *idleConnTimeout,
		DialTimeout:     *dialTimeout,
		AuthClient:      vaultClient,
	})
	listenString := fmt.Sprintf(":%v", *portFlag)
	log.Printf("Listening on %v\n", listenString)
	log.Printf("- Forwarding Traffic to '%s'\n", targetURL)
	log.Printf("- Using Credentials from from Vault '%s' with credentialsPath '%s'\n", e.VaultUrl, e.VaultCredentialsPath)

	log.Fatal(http.ListenAndServe(listenString, signingProxy))
}
