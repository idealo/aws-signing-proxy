aws-signing-proxy
=================

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

The primary use case for this program is as a credential provider for AWS tooling. Using it is a two-step process:

1. Export the required env variables:
```
export ASP_VAULT_AUTH_TOKEN=someTokenWhichAllowsYouToAccessVault; \
export ASP_VAULT_URL=https://vault.url.invalid; \
export ASP_TARGET_URL=https://someAWSServiceSupportingSignedHttpRequests; \
```
2. Create an aws config file with the following contents:
[some-aws-profile-name]
credential_process = /path/to/vault-env-cred-provider

### Docker
You can find the built image at: https://hub.docker.com/repository/docker/roechi/aws-signing-proxy
Make sure to provide all required ENV variables (`ASP_VAULT_AUTH_TOKEN`, `ASP_VAULT_URL`, `ASP_TARGET_URL`, `ASP_SERVICE`, `AWS_REGION`, `ASP_VAULT_CREDENTIALS_PATH`).

## License

This project is based on https://github.com/cllunsford/aws-signing-proxy which is licensed as follows:

MIT 2018 (c) Chris Lunsford 

