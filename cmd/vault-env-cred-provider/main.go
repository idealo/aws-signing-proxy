package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/roechi/aws-signing-proxy/pkg/vault"
	"log"
	"net/url"
	"time"
)

type EnvConfig struct {
	TargetUrl            string `split_words:"true"`
	Port                 int    `default:"8080"`
	Service              string `default:"es"`
	VaultUrl             string `split_words:"true"` // 'https://vaulthost'
	VaultAuthToken       string `split_words:"true"` // auth-token for accessing Vault
	VaultCredentialsPath string `split_words:"true"` // path were aws credentials can be generated/retrieved (e.g: 'aws/creds/my-role')
}

type RefreshedCredentials struct {
	LeaseDuration int `json:"lease_duration"`
	Data          struct {
		AccessKey     string `json:"access_key"`
		SecretKey     string `json:"secret_key"`
		SecurityToken string `json:"security_token"`
	} `json:"data"`
}

type OutputCredentials struct {
	Version         int    `json:"Version"`
	AccessKeyId     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken"`
	Expiration      string `json:"Expiration"`
}

func main() {
	// Adding envconfig to allow setting key vars via ENV
	var e EnvConfig
	if err := envconfig.Process("ASP", &e); err != nil {
		log.Fatal(err.Error())
	}

	var urlFlag = flag.String("url", e.VaultUrl, "base url of vault e.g. 'https://foo.vault.invalid'")
	var pathFlag = flag.String("path", e.VaultCredentialsPath, "path for credentials e.g. '/some-aws-engine/creds/some-aws-role'")
	var authTokenFlag = flag.String("token", e.VaultAuthToken, "token for authenticating with vault (NOTE: use the env variable ASP_VAULT_AUTH_TOKEN instead)")

	flag.Parse()

	// Validate vault base Url
	if len(*urlFlag) == 0 {
		log.Fatal("requires base URL to vault. please use the -url flag or the env variable")
	}
	_, err := url.Parse(*urlFlag)
	if err != nil {
		log.Fatal(err)
	}

	if len(*pathFlag) == 0 {
		log.Fatal("requires path for vault secret. please use the -path flag or the env variable")
	}

	if len(*authTokenFlag) == 0 {
		log.Fatal("requires auth token for vault. please use the -token flag or the env variable")
	}

	fetchedCredentials := &RefreshedCredentials{}

	err = vault.NewVaultClient().
		WithBaseUrl(*urlFlag).
		WithToken(*authTokenFlag).
		Read(*pathFlag).Into(fetchedCredentials)
	if err != nil {
		log.Fatal(err)
	}

	expiration := time.Now().Add((time.Duration(fetchedCredentials.LeaseDuration) - 60) * time.Second)
	output := &OutputCredentials{
		Version:         1,
		AccessKeyId:     fetchedCredentials.Data.AccessKey,
		SecretAccessKey: fetchedCredentials.Data.SecretKey,
		SessionToken:    fetchedCredentials.Data.SecurityToken,
		Expiration:      expiration.Format(time.RFC3339),
	}

	b, err := json.Marshal(output)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))
}
