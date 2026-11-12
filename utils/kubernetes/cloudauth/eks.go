package cloudauth

import (
	"context"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"golang.org/x/oauth2"
)

const (
	// eksTokenPrefix marks a bearer token as a presigned STS URL for the
	// aws-iam-authenticator webhook built into EKS.
	eksTokenPrefix = "k8s-aws-v1."
	// eksClusterIDHeader binds the presigned URL to one cluster, so a token
	// minted for one cluster cannot be replayed against another.
	eksClusterIDHeader = "x-k8s-aws-id"
	// eksPresignExpiresSeconds is the X-Amz-Expires value every EKS token
	// client signs. EKS ignores it and accepts the URL for eksTokenLifetime
	// after X-Amz-Date.
	eksPresignExpiresSeconds = "60"
	// eksTokenLifetime is how long EKS accepts a token after it is signed.
	eksTokenLifetime = 15 * time.Minute
	// awsCredentialCheckTimeout bounds the credential retrieval done at
	// setup, which may walk the whole default chain (including instance
	// metadata probes that time out off EC2) or call STS AssumeRole.
	awsCredentialCheckTimeout = 15 * time.Second
)

// NewEKSTokenSource resolves AWS credentials for cfg through the AWS SDK
// default chain (plus an STS AssumeRole when cfg.RoleARN is set), verifies
// that they can be retrieved, and returns a token source that mints and
// refreshes EKS bearer tokens for cfg.ClusterID.
//
// It returns ErrAWSCredentialsUnavailable when no usable credentials are
// available to the process, and ErrEKSRegionUnresolved when no region is
// configured anywhere.
func NewEKSTokenSource(ctx context.Context, cfg EKSConfig) (oauth2.TokenSource, error) {
	awsCfg, err := loadAWSConfig(ctx, cfg)
	if err != nil {
		return nil, ErrAWSCredentialsUnavailable(err, cfg.ClusterID)
	}
	if awsCfg.Region == "" {
		return nil, ErrEKSRegionUnresolved(cfg.ClusterID)
	}
	if awsCfg.Credentials == nil {
		return nil, ErrAWSCredentialsUnavailable(errNoCredentialProvider, cfg.ClusterID)
	}

	checkCtx, cancel := context.WithTimeout(ctx, awsCredentialCheckTimeout)
	defer cancel()
	if _, err := awsCfg.Credentials.Retrieve(checkCtx); err != nil {
		return nil, ErrAWSCredentialsUnavailable(err, cfg.ClusterID)
	}

	return newEKSTokenSource(awsCfg, cfg.ClusterID), nil
}

// NewEKSTokenSourceFromConfig returns a token source that mints and
// refreshes EKS bearer tokens for clusterID with the credentials and region
// of awsCfg. It performs no credential check, so callers that already hold
// an aws.Config (for example one built from credentials stored with a cloud
// connection) can use it directly.
func NewEKSTokenSourceFromConfig(awsCfg aws.Config, clusterID string) oauth2.TokenSource {
	return newEKSTokenSource(awsCfg, clusterID)
}

func newEKSTokenSource(awsCfg aws.Config, clusterID string) *cachingTokenSource {
	presignClient := sts.NewPresignClient(sts.NewFromConfig(awsCfg))
	return newCachingTokenSource(func(ctx context.Context, now time.Time) (*oauth2.Token, error) {
		token, err := presignEKSToken(ctx, presignClient, clusterID)
		if err != nil {
			return nil, ErrMintEKSToken(err, clusterID)
		}
		return &oauth2.Token{
			AccessToken: token,
			TokenType:   "Bearer",
			Expiry:      now.Add(eksTokenLifetime),
		}, nil
	})
}

// presignEKSToken presigns an STS GetCallerIdentity GET request with the
// cluster ID header signed in, and encodes the URL as an EKS bearer token.
// Presigning is a local computation; it makes no network call unless the
// credentials themselves need refreshing.
func presignEKSToken(ctx context.Context, client *sts.PresignClient, clusterID string) (string, error) {
	request, err := client.PresignGetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}, func(o *sts.PresignOptions) {
		o.Presigner = eksPresigner{base: o.Presigner, clusterID: clusterID}
	})
	if err != nil {
		return "", err
	}
	return eksTokenPrefix + base64.RawURLEncoding.EncodeToString([]byte(request.URL)), nil
}

// eksPresigner adds the headers EKS token validation requires before
// delegating to the SDK's SigV4 presigner. The signer keeps x-k8s-aws-id as
// a signed header and hoists X-Amz-Expires into the query string.
type eksPresigner struct {
	base      sts.HTTPPresignerV4
	clusterID string
}

func (p eksPresigner) PresignHTTP(
	ctx context.Context, credentials aws.Credentials, r *http.Request,
	payloadHash string, service string, region string, signingTime time.Time,
	optFns ...func(*v4.SignerOptions),
) (string, http.Header, error) {
	r.Header.Set(eksClusterIDHeader, p.clusterID)
	r.Header.Set("X-Amz-Expires", eksPresignExpiresSeconds)
	return p.base.PresignHTTP(ctx, credentials, r, payloadHash, service, region, signingTime, optFns...)
}

// loadAWSConfig builds the aws.Config for cfg: the SDK default chain,
// narrowed by the profile, region, shared files and static keys parsed from
// the exec stanza, with an AssumeRole provider layered on top when the
// stanza names a role.
func loadAWSConfig(ctx context.Context, cfg EKSConfig) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error
	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}
	if cfg.Profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(cfg.Profile))
	}
	if cfg.ConfigFile != "" {
		opts = append(opts, config.WithSharedConfigFiles([]string{cfg.ConfigFile}))
	}
	if cfg.CredentialsFile != "" {
		opts = append(opts, config.WithSharedCredentialsFiles([]string{cfg.CredentialsFile}))
	}
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken)))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, err
	}

	if cfg.RoleARN != "" {
		provider := stscreds.NewAssumeRoleProvider(sts.NewFromConfig(awsCfg), cfg.RoleARN,
			func(o *stscreds.AssumeRoleOptions) {
				if cfg.RoleSessionName != "" {
					o.RoleSessionName = cfg.RoleSessionName
				}
				if cfg.ExternalID != "" {
					o.ExternalID = aws.String(cfg.ExternalID)
				}
			})
		awsCfg.Credentials = aws.NewCredentialsCache(provider)
	}
	return awsCfg, nil
}
