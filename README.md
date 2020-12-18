aws-signing-proxy
=================

# Build & Run

## Local

### Build

1. Change directory to `cmd/aws-signing-proxy`
2. Run `go build`

### Run

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

### Docker
You can find the built image at: https://hub.docker.com/repository/docker/roechi/aws-signing-proxy
Make sure to provide all required ENV variables (`ASP_VAULT_AUTH_TOKEN`, `ASP_VAULT_URL`, `ASP_TARGET_URL`, `ASP_SERVICE`, `AWS_REGION`, `ASP_VAULT_CREDENTIALS_PATH`).

## License

This project is based on https://github.com/cllunsford/aws-signing-proxy which is licensed as follows:

MIT 2018 (c) Chris Lunsford 

