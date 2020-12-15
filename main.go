package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client/metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/kelseyhightower/envconfig"
)

type EnvConfig struct {
	Target     string
	Port       int    `default:"8080"`
	Service    string `default:"es"`
	Vault      string
	VaultToken string
}

type AppConfig struct {
	Service         string
	FlushInterval   time.Duration
	IdleConnTimeout time.Duration
	DialTimeout     time.Duration
}

type GeneratedVaultCreds struct {
	LeaseDuration int `json:"lease_duration"`
	Data          struct {
		AccessKey     string `json:"access_key"`
		SecretKey     string `json:"secret_key"`
		SecurityToken string `json:"security_token"`
	} `json:"data"`
}

// NewSigningProxy proxies requests to AWS services which require URL signing using the provided credentials
func NewSigningProxy(target *url.URL, credentials *credentials.Credentials, region string, appConfig AppConfig) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		// Rewrite request to desired server host
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host

		if _, err := credentials.Get(); err != nil {
			// We couldn't get any credentials
			log.Panic(err)
			return
		}

		// To perform the signing, we leverage aws-sdk-go
		// aws.request performs more functions than we need here
		// we only populate enough of the fields to successfully
		// sign the request
		config := aws.NewConfig().WithCredentials(credentials).WithRegion(region)

		clientInfo := metadata.ClientInfo{
			ServiceName: appConfig.Service,
		}

		operation := &request.Operation{
			Name:       "",
			HTTPMethod: req.Method,
			HTTPPath:   req.URL.Path,
		}

		handlers := request.Handlers{}
		handlers.Sign.PushBack(v4.SignSDKRequest)

		// Do we need to use request.New ? Or can we create a raw Request struct and
		//  jus swap out the HTTPRequest with our own existing one?
		awsReq := request.New(*config, clientInfo, handlers, nil, operation, nil, nil)
		// Referenced during the execution of awsReq.Sign():
		//  req.Config.Credentials
		//  req.Config.LogLevel.Value()
		//  req.Config.Logger
		//  req.ClientInfo.SigningRegion (will default to Config.Region)
		//  req.ClientInfo.SigningName (will default to ServiceName)
		//  req.ClientInfo.ServiceName
		//  req.HTTPRequest
		//  req.Time
		//  req.ExpireTime
		//  req.Body

		// Set the body in the awsReq for calculation of body Digest
		// iotuil.ReadAll reads the Body from the stream so it can be copied into awsReq
		// This drains the body from the original (proxied) request.
		// To fix, we replace req.Body with a copy (NopCloser provides io.ReadCloser interface)
		if req.Body != nil {
			buf, err := ioutil.ReadAll(req.Body)
			if err != nil {
				log.Printf("error reading request body: %v\n", err)
			}
			req.Body = ioutil.NopCloser(bytes.NewBuffer(buf))

			awsReq.SetBufferBody(buf)
		}

		// Use the updated req.URL for creating the signed request
		// We pass the full URL object to include Host, Scheme, and any params
		awsReq.HTTPRequest.URL = req.URL
		// These are now set above via req, but it's imperative that this remains
		//  correctly set before calling .Sign()
		//awsReq.HTTPRequest.URL.Scheme = target.Scheme
		//awsReq.HTTPRequest.URL.Host = target.Host

		// Perform the signing, updating awsReq in place
		if err := awsReq.Sign(); err != nil {
			log.Printf("error signing: %v\n", err)
		}

		// Write the Signed Headers into the Original Request
		for k, v := range awsReq.HTTPRequest.Header {
			req.Header[k] = v
		}
	}

	// transport is http.DefaultTransport but with the ability to override some
	// timeouts
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   appConfig.DialTimeout,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     appConfig.IdleConnTimeout,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &httputil.ReverseProxy{
		Director:      director,
		FlushInterval: appConfig.FlushInterval,
		Transport:     transport,
	}
}

func main() {
	// Adding envconfig to allow setting key vars via ENV
	var e EnvConfig
	if err := envconfig.Process("ASP", &e); err != nil {
		log.Fatal(err.Error())
	}

	var targetFlag = flag.String("target", e.Target, "target url to proxy to")
	var portFlag = flag.Int("port", e.Port, "listening port for proxy")
	var serviceFlag = flag.String("service", e.Service, "AWS Service.")
	var regionFlag = flag.String("region", os.Getenv("AWS_REGION"), "AWS region for credentials")
	var flushInterval = flag.Duration("flush-interval", 0, "Flush interval to flush to the client while copying the response body.")
	var idleConnTimeout = flag.Duration("idle-conn-timeout", 90*time.Second, "the maximum amount of time an idle (keep-alive) connection will remain idle before closing itself. Zero means no limit.")
	var dialTimeout = flag.Duration("dial-timeout", 30*time.Second, "The maximum amount of time a dial will wait for a connect to complete.")

	flag.Parse()

	appC := AppConfig{
		Service:         *serviceFlag,
		FlushInterval:   *flushInterval,
		IdleConnTimeout: *idleConnTimeout,
		DialTimeout:     *dialTimeout,
	}

	// Validate target URL
	if len(*targetFlag) == 0 {
		log.Fatal("Requires target URL to proxy to. Please use the -target flag")
	}
	targetURL, err := url.Parse(*targetFlag)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Get credentials:
	// Environment variables > local aws config file > remote role provider
	// https://github.com/aws/aws-sdk-go/blob/master/aws/defaults/defaults.go#L88
	creds := NewVaultCredChain(e)
	if _, err = creds.Get(); err != nil {
		// We couldn't get any credentials
		log.Panic(err)
		return
	}

	// Region order of precident:
	// regionFlag > os.Getenv("AWS_REGION") > "eu-central-1"
	region := *regionFlag
	if len(region) == 0 {
		region = "eu-central-1"
	}

	// Start the proxy server
	credentials := NewVaultCredChain(e)
	proxy := NewSigningProxy(targetURL, credentials, region, appC)
	listenString := fmt.Sprintf(":%v", *portFlag)
	log.Printf("Listening on %v\n", listenString)
	http.ListenAndServe(listenString, proxy)
}

func fetchSTSCredentials(e EnvConfig) (*GeneratedVaultCreds, error) {
	generatedVaultCreds := &GeneratedVaultCreds{}
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/al-postman/creds/hackday-proxy", e.Vault), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("X-Vault-Token", e.VaultToken)
	r, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if r.StatusCode > 299 {
		return nil, fmt.Errorf("encountered: %d", r.StatusCode)
	}
	err = json.NewDecoder(r.Body).Decode(generatedVaultCreds)
	if err != nil {
		return nil, err
	}

	return generatedVaultCreds, nil
}

func NewVaultCredChain(e EnvConfig) *credentials.Credentials {
	verboseErrors := false
	return credentials.NewCredentials(&credentials.ChainProvider{
		VerboseErrors: aws.BoolValue(&verboseErrors),
		Providers: []credentials.Provider{
			&VaultCredentialProvider{EnvConfig: e},
		},
	})
}

type VaultCredentialProvider struct {
	ExpirationDate  time.Time
	EnvConfig       EnvConfig
	AccessKey       string
	SecretAccessKey string
	SessionToken    string
}

func (v *VaultCredentialProvider) Retrieve() (credentials.Value, error) {
	res, err := fetchSTSCredentials(v.EnvConfig)
	if err != nil {
		log.Fatal(err)
	}

	v.ExpirationDate = time.Now().Add((time.Duration(res.LeaseDuration) - 60) * time.Second)

	return credentials.Value{
		AccessKeyID:     res.Data.AccessKey,
		SecretAccessKey: res.Data.SecretKey,
		SessionToken:    res.Data.SecurityToken,
	}, nil
}

func (v *VaultCredentialProvider) IsExpired() bool {
	return time.Now().After(v.ExpirationDate)
}
