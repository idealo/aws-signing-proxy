package irsa

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	. "github.com/idealo/aws-signing-proxy/pkg/logging"
	"github.com/idealo/aws-signing-proxy/pkg/proxy"
	"go.uber.org/zap"
	"os"
	"time"
)

var cachedCredentials *sts.Credentials

type ReadClient struct {
	stsClient stsiface.STSAPI
	clientId  string
	roleArn   string
}

func NewIRSAClient(region string, clientId string, roleArn string) *ReadClient {
	return &ReadClient{
		stsClient: InitClient(region),
		clientId:  clientId,
		roleArn:   roleArn,
	}
}

func (c *ReadClient) WithRoleArn(roleArn string) *ReadClient {
	c.roleArn = roleArn
	return c
}

func (c *ReadClient) retrieveShortLivingCredentialsFromAwsSts(roleArn string, webToken string, roleSessionName string) *sts.Credentials {
	identity, err := c.stsClient.AssumeRoleWithWebIdentity(&sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          &roleArn,
		RoleSessionName:  &roleSessionName,
		WebIdentityToken: &webToken,
	})

	if err != nil {
		Logger.Error("Something went wrong with the STS Client", zap.Error(err))
	}

	return identity.Credentials
}

func InitClient(region string) stsiface.STSAPI {
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.AnonymousCredentials},
	))

	return sts.New(sess, aws.NewConfig().WithRegion(region))
}

func (c *ReadClient) RefreshCredentials(result interface{}) error {
	refreshedCredentials := result.(*proxy.RefreshedCredentials)

	err := RetrieveCredentials(c)
	if err != nil {
		return err
	}
	stsCredentials := cachedCredentials

	refreshedCredentials.ExpiresAt = *stsCredentials.Expiration
	refreshedCredentials.Data.AccessKey = *stsCredentials.AccessKeyId
	refreshedCredentials.Data.SecretKey = *stsCredentials.SecretAccessKey
	refreshedCredentials.Data.SecurityToken = *stsCredentials.SessionToken

	return nil
}

func RetrieveCredentials(c *ReadClient) error {
	if cachedCredentials == nil || isExpired(cachedCredentials.Expiration) {

		tokenFile, present := os.LookupEnv("AWS_WEB_IDENTITY_TOKEN_FILE")
		if !present {
			zap.S().Errorf("IRSA token file is missing.")
		}

		bytes, err := os.ReadFile(tokenFile)
		if err != nil {
			return err
		}

		cachedCredentials = c.retrieveShortLivingCredentialsFromAwsSts(c.roleArn, string(bytes), c.clientId)
		Logger.Info("Refreshed short living credentials.")
	}
	return nil
}

func isExpired(expiration *time.Time) bool {
	// subtract 5 minutes from the actual expiration to retrieve every 55 minutes new credentials
	return time.Now().After(expiration.Add(-time.Minute * 5))
}
