aws-signing-proxy
=================
![Github Package](https://github.com/idealo/aws-signing-proxy/workflows/goreleaser/badge.svg)
![Docker Image CI](https://github.com/idealo/aws-signing-proxy/workflows/Docker%20Image%20CI/badge.svg)

A transparent proxy which forwards and signs http requests to AWS services.

Supported AWS credentials:

* [Static environment based AWS credentials](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html#envvars-set)
* [AWS credential files](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html#cli-configure-files-where)
* Fetching short-lived credentials from a vault set up with
  an [AWS secrets engine & sts-assumerole](https://www.vaultproject.io/docs/secrets/aws#sts-assumerole)
* Fetching short-lived credentials from AWS via a OAuth2 authorization server
  and [OpenID Connect (OIDC)](https://openid.net/connect/)
  * Additionally, you can fetch these credentials asynchronously
* Fetching short-lived credentials via AWS [IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) (IAM Roles for Service Accounts)

For ready-to-use binaries have a look at [Releases](https://github.com/idealo/aws-signing-proxy/releases).

Additionally, we provide a [Docker image](https://hub.docker.com/r/idealo/aws-signing-proxy) which can be used as a sidecar in Kubernetes.


## 🎉 Version 2.1.0 Update 🎉
* Support for AWS [IRSA](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)

## 🎉 Version 2.0.0 Update 🎉

* Version 2.0.0 comes 
  * with a built-in circuit breaker for requesting credentials from either OIDC or Vault 
  * with better error handling and panic recovery
  * with json logging enabled by default

### Breaking Changes

* Command line flags are now kebab-case to be POSIX style guide compliant
* `Health Port` is now called `Mgmt Port` 
  * it provides the `/status/health` endpoint for health probes and `/status/metrics` endpoint for prometheus metrics

# Build & Run

## Local

### Building

#### aws-signing-proxy

1. Change directory to `cmd/aws-signing-proxy`
2. Run `go build`

### Running

❗NOTE: the provided pre-built macOS binaries might fail with name resolution issues on your OSX machine if you are
using a (corporate) VPN. This will not occur on linux/windows/docker. If you are affected: either use the provided
docker image or build the binaries on your machine from source.

#### With Credentials via Vault

Execute the binary with the required environment variables set:

```
ASP_CREDENTIALS_PROVIDER=vault; \
ASP_VAULT_AUTH_TOKEN=someTokenWhichAllowsYouToAccessVault; \
ASP_VAULT_URL=https://vault.url.invalid; \
ASP_TARGET_URL=https://someAWSServiceSupportingSignedHttpRequests; \
ASP_SERVICE=s3; \
AWS_REGION=eu-central-1; \
ASP_VAULT_CREDENTIALS_PATH=/an-aws-engine-in-vault/creds/a-role-defined-aws; \
aws-signing-proxy
```

#### With Credentials via OIDC

Execute the binary with either the required environment variables set or via cli flags:

```
ASP_CREDENTIALS_PROVIDER=oidc; \
ASP_TARGET_URL=https://someAWSServiceSupportingSignedHttpRequests; \
ASP_ROLE_ARN=arn:aws:iam::123456242:role/some-access-role; \
ASP_OPEN_ID_AUTH_SERVER_URL="https://your-oauth2-authorization-server/eg/aws/token/"; \
ASP_OPEN_ID_CLIENT_ID=your-oauth2-client; \
ASP_OPEN_ID_CLIENT_SECRET=someverysecurepassword; \
aws-signing-proxy
```

#### With Credentials via IRSA (IAM Roles for Service Accounts)

Execute the binary with either the required environment variables set or via cli flags:

```
ASP_CREDENTIALS_PROVIDER=irsa; \
ASP_TARGET_URL=https://someAWSServiceSupportingSignedHttpRequests; \
ASP_ROLE_ARN=arn:aws:iam::123456242:role/some-access-role; \
aws-signing-proxy
```

Make sure, your AWS_WEB_IDENTITY_TOKEN_FILE environment variable is set!

#### Adjusting the Circuit Breaker Behaviour

If you want to adjust the built-in authorization server circuit breaker, you can set the following environment variables according to your needs. 

The failure threshold defaults to 5 failed requests until the circuit is opened
The timeout for keeping the circuit open defaults to 60s

`ASP_CIRCUIT_BREAKER_FAILURE_THRESHOLD=5`

`ASP_CIRCUIT_BREAKER_TIMEOUT=60s`

#### Fetching OIDC Credentials asynchronously

Sometimes it is crucial to have the credentials refreshed in the background to avoid a delay for the first-fetch-request

You can enable this feature by setting the environment variable `ASP_ASYNC_OPEN_ID_CREDENTIALS_FETCH` or the flag `--async-open-id-creds-fetch` to true.

It will check every 10 seconds if the credentials are still valid and takes care of refreshing them in the background.

#### Configure the Management Port and Metrics Path

If you want to alter the default port `8081` for the `/status/health` and the `/status/metrics` path, you can do that via setting the environment variable `ASP_MGMT_PORT` or the flag `--mgmt-port` to the port you like.

To alter the prometheus metrics path, you can set the environment variable `ASP_METRICS_PATH` or use the flag `--metrics-path`

### Docker

You can find the built image at: https://hub.docker.com/r/idealo/aws-signing-proxy

## Acknowledgement

This project is based on https://github.com/cllunsford/aws-signing-proxy which is licensed as follows:

MIT 2018 (c) Chris Lunsford 

