package cloudauth

import (
	"strconv"
	"strings"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Provider identifies the managed Kubernetes service an exec stanza
// authenticates against.
type Provider string

const (
	// ProviderEKS is Amazon Elastic Kubernetes Service.
	ProviderEKS Provider = "eks"
	// ProviderGKE is Google Kubernetes Engine.
	ProviderGKE Provider = "gke"
)

// Exec stanza environment variables honored for EKS. They only narrow which
// of the Meshery process's credentials are used; they never supply
// credentials of their own.
const (
	envAWSProfile       = "AWS_PROFILE"
	envAWSRegion        = "AWS_REGION"
	envAWSDefaultRegion = "AWS_DEFAULT_REGION"
)

// awsIdentityEnv lists exec stanza environment variables that make the
// plugin authenticate as an identity other than the Meshery process's own:
// static keys, a role or web identity token, alternative shared config
// files, a container credentials endpoint, or the AWS CLI's
// AWS_DEFAULT_PROFILE. A stanza carrying any of them, whatever the value, is
// left on the exec plugin, because minting in process would silently
// replace that identity with the process's.
var awsIdentityEnv = []string{
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
	"AWS_SESSION_TOKEN",
	"AWS_ROLE_ARN",
	"AWS_ROLE_SESSION_NAME",
	"AWS_WEB_IDENTITY_TOKEN_FILE",
	"AWS_CONFIG_FILE",
	"AWS_SHARED_CREDENTIALS_FILE",
	"AWS_CONTAINER_CREDENTIALS_FULL_URI",
	"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI",
	"AWS_DEFAULT_PROFILE",
}

// gkeADCFlag is the gke-gcloud-auth-plugin flag that makes the plugin use
// Application Default Credentials. Without it the plugin runs
// "gcloud config config-helper" and uses the gcloud CLI's active account,
// an identity Application Default Credentials cannot reproduce.
const gkeADCFlag = "--use_application_default_credentials"

// awsCLISessionName is the role session name "aws eks get-token" uses when it
// assumes --role-arn, kept so CloudTrail entries look the same either way.
const awsCLISessionName = "EKSGetTokenAuth"

// Spec is a recognized cloud exec credential plugin stanza.
type Spec struct {
	// Provider is the managed Kubernetes service the stanza targets.
	Provider Provider
	// Command is the exec stanza's command, unmodified. It is what the
	// Authenticator looks up on PATH when deciding whether it can fall back
	// to the exec plugin.
	Command string
	// EKS holds the parsed EKS settings. It is the zero value unless
	// Provider is ProviderEKS.
	EKS EKSConfig
}

// EKSConfig is everything needed to mint EKS tokens for one cluster.
type EKSConfig struct {
	// ClusterID is the value signed into the x-k8s-aws-id header: the
	// cluster name, or the cluster ID for EKS local clusters on Outposts.
	ClusterID string
	// Region is the AWS region of the cluster and of the STS endpoint. Empty
	// means "resolve from the process environment or shared config"; the
	// Authenticator fills it from the cluster endpoint's host name.
	Region string
	// Profile is the shared config profile to load. Empty means the default
	// profile, or AWS_PROFILE from the process environment.
	Profile string
	// RoleARN, when set, is assumed through STS with the chain credentials.
	RoleARN string
	// RoleSessionName is the session name used when assuming RoleARN.
	RoleSessionName string
	// ExternalID is passed to STS AssumeRole when assuming RoleARN.
	ExternalID string
}

// cacheKey identifies the token source a Spec resolves to, so that every
// rest.Config built for the same cluster identity shares one source and one
// refresh schedule.
func (s Spec) cacheKey() string {
	switch s.Provider {
	case ProviderEKS:
		e := s.EKS
		return strings.Join([]string{
			string(s.Provider), e.ClusterID, e.Region, e.Profile, e.RoleARN,
			e.RoleSessionName, e.ExternalID,
		}, "\x00")
	default:
		// ADC is process-wide, so every GKE stanza shares one source.
		return string(s.Provider)
	}
}

// Recognize reports whether exec invokes a supported cloud credential plugin
// in a way the Meshery process's own credentials reproduce and, if so,
// returns its parsed settings. Anything else returns false so the caller
// leaves the stanza to client-go: an unknown command; a recognized command
// whose arguments do not describe a token request (for example
// "aws s3 ls", or "aws eks get-token" without a cluster name); malformed
// flags; an EKS stanza whose env selects its own AWS identity (see
// awsIdentityEnv); a gke-gcloud-auth-plugin stanza that does not select
// Application Default Credentials; and every gcloud stanza, because
// "gcloud config config-helper" uses the gcloud CLI's active account.
//
// Recognize looks only at the exec stanza. Whether the cluster endpoint may
// receive the process's credentials is decided separately by the
// Authenticator.
func Recognize(exec *clientcmdapi.ExecConfig) (Spec, bool) {
	if exec == nil {
		return Spec{}, false
	}

	switch commandBase(exec.Command) {
	case "aws":
		if hasAnyEnv(exec.Env, awsIdentityEnv) {
			return Spec{}, false
		}
		cfg, ok := parseAWSCLI(exec.Args)
		if !ok {
			return Spec{}, false
		}
		applyAWSExecEnv(&cfg, exec.Env)
		return Spec{Provider: ProviderEKS, Command: exec.Command, EKS: cfg}, true
	case "aws-iam-authenticator":
		if hasAnyEnv(exec.Env, awsIdentityEnv) {
			return Spec{}, false
		}
		cfg, ok := parseAWSIAMAuthenticator(exec.Args)
		if !ok {
			return Spec{}, false
		}
		applyAWSExecEnv(&cfg, exec.Env)
		return Spec{Provider: ProviderEKS, Command: exec.Command, EKS: cfg}, true
	case "gke-gcloud-auth-plugin":
		if !selectsApplicationDefaultCredentials(exec) {
			return Spec{}, false
		}
		return Spec{Provider: ProviderGKE, Command: exec.Command}, true
	default:
		return Spec{}, false
	}
}

// selectsApplicationDefaultCredentials reports whether a
// gke-gcloud-auth-plugin stanza authenticates with exactly the Application
// Default Credentials the Meshery process uses: gkeADCFlag is set (bare or
// with a true value), no other argument is present, and the stanza sets no
// environment variables. Any other argument (for example
// --impersonate_service_account or --use_edge_cloud) or env entry (for
// example GOOGLE_APPLICATION_CREDENTIALS, CLOUDSDK_CONFIG or HOME, which
// move the ADC file the plugin reads) could select a different identity, so
// such stanzas stay on the plugin.
func selectsApplicationDefaultCredentials(exec *clientcmdapi.ExecConfig) bool {
	if len(exec.Env) > 0 || len(exec.Args) == 0 {
		return false
	}
	useADC := false
	for _, arg := range exec.Args {
		name, value, hasValue := strings.Cut(arg, "=")
		if name != gkeADCFlag {
			return false
		}
		useADC = true
		if hasValue {
			enabled, err := strconv.ParseBool(value)
			if err != nil {
				return false
			}
			useADC = enabled
		}
	}
	return useADC
}

// hasAnyEnv reports whether env sets any of names, whatever the value.
func hasAnyEnv(env []clientcmdapi.ExecEnvVar, names []string) bool {
	for _, e := range env {
		for _, name := range names {
			if e.Name == name {
				return true
			}
		}
	}
	return false
}

// commandBase returns the lowercase basename of command without a Windows
// executable suffix. Both "/" and "\" separate path elements, because a
// kubeconfig written on Windows may be read on any platform.
func commandBase(command string) string {
	base := command
	if i := strings.LastIndexAny(base, `/\`); i >= 0 {
		base = base[i+1:]
	}
	base = strings.ToLower(base)
	for _, suffix := range []string{".exe", ".cmd", ".bat"} {
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix)
		}
	}
	return base
}

// parseAWSCLI parses the arguments of "aws eks get-token". The subcommand
// must appear as the adjacent pair "eks get-token"; global flags may precede
// it.
func parseAWSCLI(args []string) (EKSConfig, bool) {
	if !hasAdjacent(args, "eks", "get-token") {
		return EKSConfig{}, false
	}

	flags, ok := parseFlags(args, map[string]string{
		"--cluster-name": "cluster-name",
		"--cluster-id":   "cluster-id",
		"--region":       "region",
		"--role-arn":     "role-arn",
		"--profile":      "profile",
	})
	if !ok {
		return EKSConfig{}, false
	}

	cfg := EKSConfig{
		ClusterID: flags["cluster-name"],
		Region:    flags["region"],
		RoleARN:   flags["role-arn"],
		Profile:   flags["profile"],
	}
	// The AWS CLI signs the Outposts local cluster ID instead of the name
	// when --cluster-id is given.
	if id := flags["cluster-id"]; id != "" {
		cfg.ClusterID = id
	}
	if cfg.RoleARN != "" {
		cfg.RoleSessionName = awsCLISessionName
	}
	return cfg, cfg.ClusterID != ""
}

// parseAWSIAMAuthenticator parses the arguments of
// "aws-iam-authenticator token".
func parseAWSIAMAuthenticator(args []string) (EKSConfig, bool) {
	if !containsArg(args, "token") {
		return EKSConfig{}, false
	}

	flags, ok := parseFlags(args, map[string]string{
		"-i":             "cluster-id",
		"--cluster-id":   "cluster-id",
		"-r":             "role",
		"--role":         "role",
		"-s":             "session-name",
		"--session-name": "session-name",
		"-e":             "external-id",
		"--external-id":  "external-id",
		"--region":       "region",
	})
	if !ok {
		return EKSConfig{}, false
	}

	cfg := EKSConfig{
		ClusterID:       flags["cluster-id"],
		Region:          flags["region"],
		RoleARN:         flags["role"],
		RoleSessionName: flags["session-name"],
		ExternalID:      flags["external-id"],
	}
	return cfg, cfg.ClusterID != ""
}

// applyAWSExecEnv folds the honored entries of the exec stanza's env list
// into cfg. Flags win over env for the region and profile, as they do for
// the CLIs themselves.
func applyAWSExecEnv(cfg *EKSConfig, env []clientcmdapi.ExecEnvVar) {
	values := make(map[string]string, len(env))
	for _, e := range env {
		values[e.Name] = e.Value
	}

	if cfg.Region == "" {
		cfg.Region = values[envAWSRegion]
	}
	if cfg.Region == "" {
		cfg.Region = values[envAWSDefaultRegion]
	}
	if cfg.Profile == "" {
		cfg.Profile = values[envAWSProfile]
	}
}

// parseFlags extracts the values of the flags named in names (flag spelling
// to canonical key) from args, accepting both "--flag value" and
// "--flag=value". The last occurrence of a flag wins.
//
// A flag whose separate value is missing, is another flag in names, or
// starts with "--" (as in "--region --cluster-name demo") makes the argument
// list malformed and parseFlags returns false: the plugin itself would
// reject it, and guessing would mint for the wrong region or cluster.
func parseFlags(args []string, names map[string]string) (map[string]string, bool) {
	values := make(map[string]string)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if name, value, found := strings.Cut(arg, "="); found {
			if key, ok := names[name]; ok {
				values[key] = value
			}
			continue
		}
		key, ok := names[arg]
		if !ok {
			continue
		}
		if i+1 >= len(args) || isFlagLike(args[i+1], names) {
			return nil, false
		}
		values[key] = args[i+1]
		i++
	}
	return values, true
}

// isFlagLike reports whether arg is a flag rather than a flag value: one of
// names (bare or as "name=value"), or any long option.
func isFlagLike(arg string, names map[string]string) bool {
	if strings.HasPrefix(arg, "--") {
		return true
	}
	name, _, _ := strings.Cut(arg, "=")
	_, known := names[name]
	return known
}

func hasAdjacent(args []string, first, second string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == first && args[i+1] == second {
			return true
		}
	}
	return false
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
