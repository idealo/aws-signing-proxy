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
	HealthPort           int    `default:"8081"`
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

	var targetFlag = flag.String("target", e.TargetUrl, "target url to proxy to (e.g. foo.eu-central-1.es.amazonaws.com)")
	var portFlag = flag.Int("port", e.Port, "listening port for proxy (e.g. 3000)")
	var healthPortFlag = flag.Int("healthPort", e.HealthPort, "Health port for proxy (e.g. 8081)")
	var serviceFlag = flag.String("service", e.Service, "AWS Service (e.g. es)")
	var vaultUrlFlag = flag.String("vaultUrl", e.VaultUrl, "base url of vault (e.g. 'https://foo.vault.invalid')")
	var vaultPathFlag = flag.String("vaultPath", e.VaultCredentialsPath, "path for credentials (e.g. '/some-aws-engine/creds/some-aws-role')")
	var vaultAuthTokenFlag = flag.String("vaultToken", e.VaultAuthToken, "token for authenticating with vault (NOTE: use the environment variable ASP_VAULT_AUTH_TOKEN instead)")

	var regionFlag = flag.String("region", os.Getenv("AWS_REGION"), "AWS region for credentials (e.g. eu-central-1)")
	var flushInterval = flag.Duration("flush-interval", 0, "non essential: flush interval to flush to the client while copying the response body.")
	var idleConnTimeout = flag.Duration("idle-conn-timeout", 90*time.Second, "non essential: the maximum amount of time an idle (keep-alive) connection will remain idle before closing itself. zero means no limit.")
	var dialTimeout = flag.Duration("dial-timeout", 30*time.Second, "non essential: the maximum amount of time a dial will wait for a connect to complete.")

	flag.Parse()

	// Validate target URL
	if anyFlagEmpty(*serviceFlag, *targetFlag) {
		log.Fatal("required parameter target (e.g. foo.eu-central-1.es.amazonaws.com) OR service (e.g. es) missing!")
	}
	targetURL, err := url.Parse(*targetFlag)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Region order of precedent:
	// regionFlag > os.Getenv("AWS_REGION") > "eu-central-1"
	region := *regionFlag
	if !anyFlagEmpty(region) {
		region = "eu-central-1"
	}

	var vaultClient *vault.ReadClient
	if anyFlagEmpty(*vaultUrlFlag, *vaultPathFlag, *vaultAuthTokenFlag) {
		log.Println("warning: disabling vault credentials source due to missing flags/environment variables!")
	} else {
		vaultClient = vault.NewVaultClient().
			WithBaseUrl(*vaultUrlFlag).
			WithToken(*vaultAuthTokenFlag).
			Read(*vaultPathFlag)
	}

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
	healthPortString := fmt.Sprintf(":%v", *healthPortFlag)
	log.Printf("Listening on %v\n", listenString)
	log.Printf("- Forwarding Traffic to '%s'\n", targetURL)
	log.Printf("- Using Credentials from from Vault '%s' with credentialsPath '%s'\n", e.VaultUrl, e.VaultCredentialsPath)

	go provideHealthEndpoint(healthPortString)

	log.Fatal(http.ListenAndServe(listenString, signingProxy))

}

func provideHealthEndpoint(h string) {
	http.HandleFunc("/status/health", func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{\"status\":\"ok\"}"))
	})
	log.Fatal(http.ListenAndServe(h, nil))
}

func anyFlagEmpty(flags ...string) bool {
	for _, flag := range flags {
		if len(flag) == 0 {
			return true
		}
	}
	return false
}
