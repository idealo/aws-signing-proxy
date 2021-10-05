package proxy

import (
	"bytes"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client/metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type Config struct {
	Target          *url.URL
	Region          string
	Service         string
	FlushInterval   time.Duration
	IdleConnTimeout time.Duration
	DialTimeout     time.Duration
	AuthClient      ReadClient
}

// NewSigningProxy proxies requests to AWS services which require URL signing using the provided credentials
func NewSigningProxy(config Config) *httputil.ReverseProxy {
	// transport is http.DefaultTransport but with the ability to override some
	// timeouts
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     config.IdleConnTimeout,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &httputil.ReverseProxy{
		Director:      director(config),
		FlushInterval: config.FlushInterval,
		Transport:     transport,
	}
}

func director(config Config) func(req *http.Request) {
	vaultCredentials := NewCredChain(config.AuthClient)

	return func(req *http.Request) {
		// Rewrite request to desired server host
		req.URL.Scheme = config.Target.Scheme
		req.URL.Host = config.Target.Host
		req.Host = config.Target.Host

		if _, err := vaultCredentials.Get(); err != nil {
			// We couldn't get any credentials
			log.Panic(err)
			return
		}

		// To perform the signing, we leverage aws-sdk-go
		// aws.request performs more functions than we need here
		// we only populate enough of the fields to successfully
		// sign the request
		c := aws.NewConfig().
			WithCredentials(vaultCredentials).
			WithRegion(config.Region)

		clientInfo := metadata.ClientInfo{
			ServiceName: config.Service,
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
		awsReq := request.New(*c, clientInfo, handlers, nil, operation, nil, nil)
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
}
