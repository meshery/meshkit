package cloudauth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	meshkiterrors "github.com/meshery/meshkit/errors"
	"golang.org/x/oauth2"
)

const (
	eksTokenPrefix            = "k8s-aws-v1."
	eksClusterIDHeader        = "x-k8s-aws-id"
	eksPresignExpiresSeconds  = "60"
	eksDefaultRegion          = "us-east-1"
	eksTokenTTL               = 15 * time.Minute
	eksRefreshLeeway          = 2 * time.Minute
	awsCredentialCheckTimeout = 15 * time.Second
	awsCLISessionName         = "EKSGetTokenAuth"
)

// AWSCredentials supplies explicit AWS credentials for in-process EKS token
// minting (e.g. when Meshery Server uses stored connection credentials instead
// of host environment files).
type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Region          string
	Profile         string
	RoleARN         string
	ExternalID      string
	RoleSessionName string
	STSEndpoint     string
}

// String redacts sensitive AWS credential fields.
func (c AWSCredentials) String() string {
	return fmt.Sprintf("cloudauth.AWSCredentials{Region:%s, Profile:%s, RoleARN:%s, HasStaticKeys:%t}",
		c.Region, c.Profile, c.RoleARN, c.AccessKeyID != "")
}

// GoString redacts sensitive AWS credential fields.
func (c AWSCredentials) GoString() string {
	return c.String()
}

type eksConfig struct {
	loadDefaultConfig func(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error)
	disableIMDS       bool
	now               func() time.Time
	expiryDelta       time.Duration
	mintTimeout       time.Duration
}

func defaultEKSConfig() eksConfig {
	return eksConfig{
		loadDefaultConfig: config.LoadDefaultConfig,
		now:               time.Now,
		expiryDelta:       eksRefreshLeeway,
		mintTimeout:       defaultMintTimeout,
	}
}

func newEKSTokenSource(ctx context.Context, match MatchedConfig, explicit *AWSCredentials, cfg eksConfig) (*RefreshingTokenSource, error) {
	if strings.TrimSpace(match.ClusterName) == "" {
		return nil, ErrInvalidCloudExecConfig(errors.New("EKS cluster name is required"))
	}

	if explicit != nil && ((explicit.AccessKeyID == "") != (explicit.SecretAccessKey == "")) {
		return nil, ErrAWSCredentials(errors.New("both AccessKeyID and SecretAccessKey are required when specifying static AWS credentials"))
	}

	awsCfg, err := loadAWSConfig(ctx, match, explicit, cfg)
	if err != nil {
		return nil, classifyAWSError(err, true)
	}
	if awsCfg.Credentials == nil {
		return nil, ErrAWSCredentials(errors.New("AWS credentials provider is nil"))
	}

	// Verify credentials can actually be resolved now so missing credentials fail
	// cleanly on kubeconfig load instead of only when the first HTTP request runs.
	checkCtx, cancel := context.WithTimeout(ctx, awsCredentialCheckTimeout)
	defer cancel()
	if _, err := awsCfg.Credentials.Retrieve(checkCtx); err != nil {
		return nil, classifyAWSError(err, true)
	}

	stsClient := sts.NewFromConfig(awsCfg)
	presignClient := sts.NewPresignClient(stsClient)

	ts := NewRefreshingTokenSource(func(callCtx context.Context) (*oauth2.Token, error) {
		if callCtx != nil {
			if err := callCtx.Err(); err != nil {
				return nil, classifyAWSError(err, false)
			}
		}
		presigned, err := presignClient.PresignGetCallerIdentity(callCtx, &sts.GetCallerIdentityInput{}, func(po *sts.PresignOptions) {
			po.ClientOptions = append(po.ClientOptions, func(o *sts.Options) {
				o.APIOptions = append(o.APIOptions,
					smithyhttp.SetHeaderValue(eksClusterIDHeader, match.ClusterName),
					smithyhttp.SetHeaderValue("X-Amz-Expires", eksPresignExpiresSeconds),
				)
			})
		})
		if err != nil {
			return nil, classifyAWSError(err, false)
		}

		token := eksTokenPrefix + base64.RawURLEncoding.EncodeToString([]byte(presigned.URL))
		return &oauth2.Token{
			AccessToken: token,
			TokenType:   "Bearer",
			Expiry:      cfg.now().Add(eksTokenTTL),
		}, nil
	}, cfg.expiryDelta)
	if cfg.now != nil {
		ts.now = cfg.now
	}
	if cfg.mintTimeout > 0 {
		ts.mintTimeout = cfg.mintTimeout
	}

	// Prime the cache immediately so signing issues surface synchronously.
	if _, err := ts.TokenWithContext(ctx); err != nil {
		return nil, classifyAWSError(err, false)
	}
	return ts, nil
}

func loadAWSConfig(ctx context.Context, match MatchedConfig, explicit *AWSCredentials, cfg eksConfig) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error

	if cfg.disableIMDS {
		opts = append(opts, config.WithEC2IMDSClientEnableState(imds.ClientDisabled))
	}

	region := resolveAWSRegion(match, explicit)
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	profile := match.Profile
	if explicit != nil {
		if explicit.Profile != "" {
			profile = explicit.Profile
		} else if explicit.AccessKeyID != "" {
			// Kubeconfig profiles belong to the author's workstation; do not require
			// a matching host profile when explicit static keys were supplied.
			profile = ""
		}
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	if cfgFile := match.Env["AWS_CONFIG_FILE"]; cfgFile != "" {
		opts = append(opts, config.WithSharedConfigFiles([]string{cfgFile}))
	}
	if credFile := match.Env["AWS_SHARED_CREDENTIALS_FILE"]; credFile != "" {
		opts = append(opts, config.WithSharedCredentialsFiles([]string{credFile}))
	}

	// Static credentials passed explicitly via WithCredentials stay strictly scoped
	// in memory (never written to process-wide env). Static AWS keys in kubeconfig
	// exec.env are deliberately ignored and not promoted into in-process credentials.
	if explicit != nil && explicit.AccessKeyID != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(explicit.AccessKeyID, explicit.SecretAccessKey, explicit.SessionToken),
		))
	}

	loader := cfg.loadDefaultConfig
	if loader == nil {
		loader = config.LoadDefaultConfig
	}
	awsCfg, err := loader(ctx, opts...)
	if err != nil {
		return aws.Config{}, err
	}

	endpoint := match.Endpoint
	if explicit != nil && explicit.STSEndpoint != "" {
		endpoint = explicit.STSEndpoint
	}
	if awsCfg.Region == "" {
		if endpoint == "" {
			return aws.Config{}, ErrEKSRegionUnresolved(match.ClusterName)
		}
		awsCfg.Region = eksDefaultRegion
	}
	if endpoint != "" {
		awsCfg.BaseEndpoint = aws.String(endpoint)
	}

	// Support IRSA when web identity vars are set via kubeconfig exec.env.
	if tokenFile, envRole := match.Env["AWS_WEB_IDENTITY_TOKEN_FILE"], match.Env["AWS_ROLE_ARN"]; tokenFile != "" && envRole != "" && (explicit == nil || explicit.AccessKeyID == "") {
		stsClient := sts.NewFromConfig(awsCfg)
		webProvider := stscreds.NewWebIdentityRoleProvider(stsClient, envRole, stscreds.IdentityTokenFile(tokenFile))
		awsCfg.Credentials = aws.NewCredentialsCache(webProvider)
	}

	// Apply explicit / exec --role-arn AssumeRole chaining on top of resolved base credentials.
	roleARN := firstNonEmpty(match.RoleARN)
	externalID := match.ExternalID
	sessionName := match.RoleSessionName
	if explicit != nil && explicit.RoleARN != "" {
		roleARN = explicit.RoleARN
		if explicit.ExternalID != "" {
			externalID = explicit.ExternalID
		}
		if explicit.RoleSessionName != "" {
			sessionName = explicit.RoleSessionName
		}
	}
	if roleARN == "" && match.Env["AWS_WEB_IDENTITY_TOKEN_FILE"] == "" && match.Env["AWS_ROLE_ARN"] != "" {
		roleARN = match.Env["AWS_ROLE_ARN"]
	}

	if roleARN != "" {
		if sessionName == "" && match.Command == "aws" {
			sessionName = awsCLISessionName
		}
		stsClient := sts.NewFromConfig(awsCfg)
		assumeProvider := stscreds.NewAssumeRoleProvider(stsClient, roleARN, func(o *stscreds.AssumeRoleOptions) {
			if externalID != "" {
				o.ExternalID = aws.String(externalID)
			}
			if sessionName != "" {
				o.RoleSessionName = sessionName
			}
		})
		awsCfg.Credentials = aws.NewCredentialsCache(assumeProvider)
	}

	return awsCfg, nil
}

func resolveAWSRegion(match MatchedConfig, explicit *AWSCredentials) string {
	if explicit != nil && strings.TrimSpace(explicit.Region) != "" {
		return explicit.Region
	}
	if strings.TrimSpace(match.Region) != "" {
		return match.Region
	}
	return firstNonEmpty(match.Env["AWS_REGION"], match.Env["AWS_DEFAULT_REGION"])
}

func classifyAWSError(err error, credentialPhase bool) error {
	if err == nil {
		return nil
	}
	var errV2 *meshkiterrors.ErrorV2
	if errors.As(err, &errV2) {
		return err
	}
	if isSSOMFAError(err) {
		return ErrUnsupportedCloudAuthFlow(err.Error())
	}
	if credentialPhase || isCredentialRetrieveError(err) {
		return ErrAWSCredentials(err)
	}
	return ErrEKSTokenMint(err)
}

func isSSOMFAError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "sso session") ||
		strings.Contains(msg, "sso token") ||
		strings.Contains(msg, "ssooidc") ||
		strings.Contains(msg, "sso provider") ||
		strings.Contains(msg, "sso login") ||
		strings.Contains(msg, "single sign-on") ||
		strings.Contains(msg, "mfa ") ||
		strings.Contains(msg, "mfa_serial") ||
		strings.Contains(msg, "multi-factor")
}

func isCredentialRetrieveError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "failed to retrieve credentials") ||
		strings.Contains(msg, "no ec2 imds role") ||
		strings.Contains(msg, "credential") ||
		strings.Contains(msg, "no valid providers")
}
