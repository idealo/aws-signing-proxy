package irsa

import (
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestRetrieveCredentialsAheadOfTime(t *testing.T) {

	readClient := ReadClient{
		stsClient: &mockStsClient{},
		clientId:  "client_id",
		roleArn:   "role_arn",
	}

	tmpToken, _ := os.CreateTemp("", "aws-irsa-token-file")
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", tmpToken.Name())

	RetrieveCredentials(&readClient)

	var accessKeyId, secretAccessKey, sessionToken string
	accessKeyId = "accessKeyId"
	secretAccessKey = "secretAccessKey"
	sessionToken = "sessionToken"
	var expiration = time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(60) * time.Minute)

	want := &sts.Credentials{
		AccessKeyId:     &accessKeyId,
		Expiration:      &expiration,
		SecretAccessKey: &secretAccessKey,
		SessionToken:    &sessionToken,
	}

	got := cachedCredentials

	if !reflect.DeepEqual(cachedCredentials, want) {
		t.Errorf("RetrieveCredentials() = %v, want %v", got, want)
	}

}

func TestRetrieveShortLivingCredentials(t *testing.T) {
	client := ReadClient{
		stsClient: &mockStsClient{},
	}

	var accessKeyId, secretAccessKey, sessionToken string
	accessKeyId = "accessKeyId"
	secretAccessKey = "secretAccessKey"
	sessionToken = "sessionToken"
	var expiration = time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(60) * time.Minute)

	want := &sts.Credentials{
		AccessKeyId:     &accessKeyId,
		Expiration:      &expiration,
		SecretAccessKey: &secretAccessKey,
		SessionToken:    &sessionToken,
	}
	got := client.retrieveShortLivingCredentialsFromAwsSts("foo", "bar", "session")

	if !reflect.DeepEqual(got, want) {
		t.Errorf("retrieveShortLivingCredentialsFromAwsSts() = %v, want %v", got, want)
	}
}

type mockStsClient struct {
	stsiface.STSAPI
}

func (*mockStsClient) AssumeRoleWithWebIdentity(*sts.AssumeRoleWithWebIdentityInput) (*sts.AssumeRoleWithWebIdentityOutput, error) {
	var accessKeyId, secretAccessKey, sessionToken string
	accessKeyId = "accessKeyId"
	secretAccessKey = "secretAccessKey"
	sessionToken = "sessionToken"
	var expiration = time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(60) * time.Minute)

	return &sts.AssumeRoleWithWebIdentityOutput{
		Credentials: &sts.Credentials{
			AccessKeyId:     &accessKeyId,
			Expiration:      &expiration,
			SecretAccessKey: &secretAccessKey,
			SessionToken:    &sessionToken,
		},
	}, nil
}
