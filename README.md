aws-signing-proxy
=================
![Github Package](https://github.com/idealo/aws-signing-proxy/workflows/goreleaser/badge.svg)
![Docker Image CI](https://github.com/idealo/aws-signing-proxy/workflows/Docker%20Image%20CI/badge.svg)

A transparent proxy which forwards and signs http requests to AWS services.
 
Supported AWS credentials:
* [Static environment based AWS credentials](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html#envvars-set)  
* [AWS credential files](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html#cli-configure-files-where)
* Fetching short-lived credentials from a vault set up with an [AWS secrets engine & sts-assumerole](https://www.vaultproject.io/docs/secrets/aws#sts-assumerole)

For ready-to-use binaries have a look at releases. Additionally, we provide a _Docker image_ which can be used both in a 
test setup and as a sidecar in kubernetes.

In addition to the proxy you may also use `vault-env-cred-provider` as an 
[credential provider for AWS tooling](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html).

❗NOTE: the provided pre-built mac os binaries might fail with name resolution issues on your apple machine if you are 
using a (corporate) VPN. This will not occur on linux/windows/docker. 
If you are affected: either use the provided docker image or build the binaries on your machine. 

# Build & Run

## Local

### Build

#### aws-signing-proxy

1. Change directory to `cmd/aws-signing-proxy`
2. Run `go build`

#### vault-env-cred-provider
1. Change directory to `cmd/vault-env-cred-provider`
2. Run `go build`

### Run

#### aws-signing-proxy

Execute the binary with the required environment variables set:
```
ASP_VAULT_AUTH_TOKEN=someTokenWhichAllowsYouToAccessVault; \
ASP_VAULT_URL=https://vault.url.invalid; \
ASP_TARGET_URL=https://someAWSServiceSupportingSignedHttpRequests; \
ASP_SERVICE=s3; \
AWS_REGION=eu-central-1; \
ASP_VAULT_CREDENTIALS_PATH=/an-aws-engine-in-vault/creds/a-role-defined-aws; \
aws-signing-proxy
```

#### vault-env-cred-provider

This program can be used as a [credential provider for AWS tooling](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html). 
Setting it up is a two-step process:

1. Export the required env variables:
```
export ASP_VAULT_AUTH_TOKEN=someTokenWhichAllowsYouToAccessVault
export ASP_VAULT_URL=https://vault.url.invalid
export ASP_VAULT_CREDENTIALS_PATH=/an-aws-engine-in-vault/creds/a-role-defined-aws
```
2. Create an aws config file with the following contents:
```
[some-aws-profile-name]
credential_process = /path/to/vault-env-cred-provider
```
3. Use AWS cli or sdk using this profile name e.g. some-aws-profile-name. 

Note that:
* You may name the AWS profile `default` so that you don't need to specify which profile to use when using the AWS SDK/CLI.
* There is no need to specify AWS_ACCESS_KEY_ID etc.

### Docker
You can find the built image at: https://hub.docker.com/repository/docker/roechi/aws-signing-proxy
Make sure to provide all required ENV variables (`ASP_VAULT_AUTH_TOKEN`, `ASP_VAULT_URL`, `ASP_TARGET_URL`, `ASP_SERVICE`, `AWS_REGION`, `ASP_VAULT_CREDENTIALS_PATH`).

## License

This project is based on https://github.com/cllunsford/aws-signing-proxy which is licensed as follows:

MIT 2018 (c) Chris Lunsford 

