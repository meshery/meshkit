package cloudauth

import (
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

// Exec stanza environment variables honored for EKS. Other entries in the
// stanza's env list, including credentials, are ignored; credentials come
// only from the Meshery process's AWS default credential chain.
const (
	envAWSProfile       = "AWS_PROFILE"
	envAWSRegion        = "AWS_REGION"
	envAWSDefaultRegion = "AWS_DEFAULT_REGION"
)

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
	// means "resolve from the process environment or shared config".
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
// and, if so, returns its parsed settings. Anything it does not recognize,
// including a recognized command whose arguments do not describe a token
// request (for example "aws s3 ls", "aws eks get-token" without a cluster
// name, or "gcloud auth print-identity-token"), returns false so the caller
// leaves the stanza to client-go.
func Recognize(exec *clientcmdapi.ExecConfig) (Spec, bool) {
	if exec == nil {
		return Spec{}, false
	}

	switch commandBase(exec.Command) {
	case "aws":
		cfg, ok := parseAWSCLI(exec.Args)
		if !ok {
			return Spec{}, false
		}
		applyAWSExecEnv(&cfg, exec.Env)
		return Spec{Provider: ProviderEKS, Command: exec.Command, EKS: cfg}, true
	case "aws-iam-authenticator":
		cfg, ok := parseAWSIAMAuthenticator(exec.Args)
		if !ok {
			return Spec{}, false
		}
		applyAWSExecEnv(&cfg, exec.Env)
		return Spec{Provider: ProviderEKS, Command: exec.Command, EKS: cfg}, true
	case "gke-gcloud-auth-plugin":
		return Spec{Provider: ProviderGKE, Command: exec.Command}, true
	case "gcloud":
		if !hasAdjacent(exec.Args, "config", "config-helper") {
			return Spec{}, false
		}
		return Spec{Provider: ProviderGKE, Command: exec.Command}, true
	default:
		return Spec{}, false
	}
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

	flags := parseFlags(args, map[string]string{
		"--cluster-name": "cluster-name",
		"--cluster-id":   "cluster-id",
		"--region":       "region",
		"--role-arn":     "role-arn",
		"--profile":      "profile",
	})

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

	flags := parseFlags(args, map[string]string{
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
func parseFlags(args []string, names map[string]string) map[string]string {
	values := make(map[string]string)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if name, value, found := strings.Cut(arg, "="); found {
			if key, ok := names[name]; ok {
				values[key] = value
			}
			continue
		}
		if key, ok := names[arg]; ok && i+1 < len(args) {
			values[key] = args[i+1]
			i++
		}
	}
	return values
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
