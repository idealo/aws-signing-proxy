package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"
)

var signedRequest *http.Request

type apiHandler struct{}

func (apiHandler) ServeHTTP(rw http.ResponseWriter, rq *http.Request) {
	signedRequest = rq
}

func TestBasicMainIntegrationTest(t *testing.T) {

	// Given

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	handleError(err)

	targetPort := strconv.Itoa(int(listener.Addr().(*net.TCPAddr).AddrPort().Port()))

	go func() {
		serveErr := http.Serve(listener, apiHandler{})
		handleError(serveErr)
	}()

	os.Setenv("ASP_TARGET_URL", "http://127.0.0.1:"+targetPort)
	os.Setenv("ASP_SERVICE", "s3")
	os.Setenv("AWS_REGION", "eu-central-1")
	os.Setenv("ASP_CREDENTIALS_PROVIDER", "awstoken")

	os.Setenv("AWS_ACCESS_KEY_ID", "FOO")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "BAR")
	os.Setenv("AWS_SESSION_TOKEN", "FOOBAR")

	// When
	go main()
	// wait a second to start the main code
	time.Sleep(time.Second * 1)

	// Then
	client := http.Client{}
	_, err = client.Get("http://127.0.0.1:8080")
	handleError(err)

	expectedSha256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if signedRequest.Header["X-Amz-Content-Sha256"][0] != expectedSha256 {
		t.Fatalf("SHA256 is wrong!\nWanted: %s\nGot: %s", expectedSha256, signedRequest.Header["X-Amz-Content-Sha256"][0])
	}

	pattern := "AWS4-HMAC-SHA256 Credential=FOO/[0-9]{8}/eu-central-1/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date;x-amz-security-token, Signature=[a-z0-9]{64}"
	compiledRegEx, err := regexp.Compile(pattern)

	if !compiledRegEx.MatchString(signedRequest.Header["Authorization"][0]) {
		log.Fatalf("Authorization Header didn't match the RegEx!\nWanted: %s\nGot: %s\n", pattern, signedRequest.Header["Authorization"][0])
	}

	if signedRequest.Header["X-Amz-Security-Token"][0] != "FOOBAR" {
		t.Fatalf("X-Amz-Security-Token is wrong!\nWanted: FOOBAR\nGot: %s", signedRequest.Header["X-Amz-Security-Token"][0])
	}

	// Basic check for verifying that the prometheus endpoint is available
	resp, err := client.Get("http://127.0.0.1:8081/status/metrics")
	if resp.StatusCode != 200 {
		t.Fatalf("Prometheus Metrics endpoint is broken!\nWanted: HTTP Status Code 200\nGot: HTTP Status Code %d", resp.StatusCode)
	}

}

func TestRequiredParamsAreChecked(t *testing.T) {
	testCases := []struct {
		envVarName string
		required   bool
	}{
		{"ASP_TARGET_URL", true},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s is required %t", tc.envVarName, tc.required), func(t *testing.T) {
			os.Setenv("ASP_TARGET_URL", "http://127.0.0.1:1337")

			os.Unsetenv(tc.envVarName)

			_, err := parseEnvironmentVariables()
			if tc.required {
				if err == nil || err.Error() != fmt.Sprintf("required key %s missing value", tc.envVarName) {
					t.Fatal(fmt.Sprintf("Fail: omitting the required parameter %s did not lead to a parsing failure.", tc.envVarName))
				}
			} else {
				if err != nil {
					handleError(err)
				}
			}
		})

	}
}

func TestRequiredParamsForOIDCAreChecked(t *testing.T) {
	testCases := []struct {
		envVarName string
		required   bool
	}{
		{"ASP_OPEN_ID_AUTH_SERVER_URL", true},
		{"ASP_OPEN_ID_CLIENT_ID", true},
		{"ASP_OPEN_ID_CLIENT_SECRET", true},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s is required %t", tc.envVarName, tc.required), func(t *testing.T) {
			os.Setenv("ASP_TARGET_URL", "http://127.0.0.1:1337")
			os.Setenv("ASP_CREDENTIALS_PROVIDER", "oidc")

			os.Setenv("ASP_OPEN_ID_AUTH_SERVER_URL", "FOORL")
			os.Setenv("ASP_OPEN_ID_CLIENT_ID", "FOO")
			os.Setenv("ASP_OPEN_ID_CLIENT_SECRET", "BAR")

			os.Unsetenv(tc.envVarName)

			_, err := parseEnvironmentVariables()
			if tc.required {
				if err == nil || err.Error() != fmt.Sprintf("required key %s missing value", tc.envVarName) {
					t.Fatal(fmt.Sprintf("Fail: omitting the required parameter %s did not lead to a parsing failure.", tc.envVarName))
				}
			} else {
				if err != nil {
					handleError(err)
				}
			}
		})

	}
}

func TestRequiredParamsForVaultAreChecked(t *testing.T) {
	requiredParams := []string{
		"ASP_VAULT_URL",
		"ASP_VAULT_PATH",
		"ASP_VAULT_AUTH_TOKEN",
	}

	for _, rp := range requiredParams {
		t.Run(fmt.Sprintf("%s is required", rp), func(t *testing.T) {
			os.Setenv("ASP_TARGET_URL", "http://127.0.0.1:1337")
			os.Setenv("ASP_CREDENTIALS_PROVIDER", "vault")

			os.Setenv("ASP_VAULT_URL", "FOORL")
			os.Setenv("ASP_VAULT_PATH", "/foo/bar")
			os.Setenv("ASP_VAULT_AUTH_TOKEN", "secret")

			os.Unsetenv(rp)

			_, err := parseEnvironmentVariables()
			if err == nil || err.Error() != fmt.Sprintf("required key %s missing value", rp) {
				t.Fatal(fmt.Sprintf("Fail: omitting the required parameter %s did not lead to a parsing failure.", rp))
			}
		})
	}
}

func TestRequiredParamsForIrsaAreChecked(t *testing.T) {
	requiredParams := []string{
		"ASP_IRSA_CLIENT_ID",
		"AWS_WEB_IDENTITY_TOKEN_FILE",
	}

	for _, rp := range requiredParams {
		t.Run(fmt.Sprintf("%s is required", rp), func(t *testing.T) {
			os.Setenv("ASP_TARGET_URL", "http://127.0.0.1:1337")
			os.Setenv("ASP_CREDENTIALS_PROVIDER", "irsa")

			os.Setenv("ASP_IRSA_CLIENT_ID", "FOO")
			os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/foo/bar")

			os.Unsetenv(rp)

			_, err := parseEnvironmentVariables()
			if err == nil || err.Error() != fmt.Sprintf("required key %s missing value", rp) {
				t.Fatal(fmt.Sprintf("Fail: omitting the required parameter %s did not lead to a parsing failure.", rp))
			}
		})
	}
}

func handleError(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}
