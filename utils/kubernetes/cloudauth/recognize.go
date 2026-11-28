package cloudauth

import (
	"crypto/sha256"
	"encoding/hex"
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
// files (directly, or by moving the home directory that holds ~/.aws), a
// container or instance metadata credentials endpoint, or the AWS CLI's
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
	"AWS_EC2_METADATA_SERVICE_ENDPOINT",
	"AWS_DEFAULT_PROFILE",
	"HOME",
	"USERPROFILE",
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
	// gkeIP is set by bindEndpoint when the server is a GKE IP endpoint.
	// It is nil for DNS endpoints and Connect Gateway, which are not
	// looked up. Recognize never sets it.
	gkeIP *gkeIPEndpoint
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
// refresh schedule. GKE DNS endpoints and Connect Gateway share one
// process-wide source, because they all use the process's Application
// Default Credentials. A GKE IP endpoint includes the host and CA in the
// key, so that endpoint is verified on its own setup and a 401 re-verifies
// it rather than reusing another cluster's setup.
func (s Spec) cacheKey() string {
	switch s.Provider {
	case ProviderEKS:
		e := s.EKS
		return strings.Join([]string{
			string(s.Provider), e.ClusterID, e.Region, e.Profile, e.RoleARN,
			e.RoleSessionName, e.ExternalID,
		}, "\x00")
	case ProviderGKE:
		if s.gkeIP == nil {
			return string(s.Provider)
		}
		sum := sha256.Sum256(s.gkeIP.CA)
		return strings.Join([]string{
			string(s.Provider), "ip", s.gkeIP.Host, hex.EncodeToString(sum[:]), s.gkeIP.CAFile,
		}, "\x00")
	default:
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
// An exec env entry named DisableEnvVar (MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH)
// with a true value is the opt-out for the kubeconfig user that owns this
// stanza, and so for every context that uses that user: Recognize returns
// false and the stanza stays on the plugin. A false or unparseable value is ignored,
// and so is the entry when deciding whether any other env var selects a
// different identity. It is not an AWS identity variable and it is not a
// GKE env entry that by itself keeps the stanza on the plugin.
//
// Recognize looks only at the exec stanza. Whether the cluster endpoint may
// receive the process's credentials is decided separately by the
// Authenticator.
func Recognize(exec *clientcmdapi.ExecConfig) (Spec, bool) {
	if exec == nil || execDisablesInProcessCloudAuth(exec) {
		return Spec{}, false
	}
	env := execEnvWithout(exec.Env, DisableEnvVar)

	switch commandBase(exec.Command) {
	case "aws":
		if hasAnyEnv(env, awsIdentityEnv) {
			return Spec{}, false
		}
		cfg, ok := parseAWSCLI(exec.Args)
		if !ok {
			return Spec{}, false
		}
		applyAWSExecEnv(&cfg, env)
		return Spec{Provider: ProviderEKS, Command: exec.Command, EKS: cfg}, true
	case "aws-iam-authenticator":
		if hasAnyEnv(env, awsIdentityEnv) {
			return Spec{}, false
		}
		cfg, ok := parseAWSIAMAuthenticator(exec.Args)
		if !ok {
			return Spec{}, false
		}
		applyAWSExecEnv(&cfg, env)
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

// execDisablesInProcessCloudAuth reports whether the exec stanza opts this
// context out of in-process minting. Only a value strconv.ParseBool accepts
// as true counts. A false value does not.
func execDisablesInProcessCloudAuth(exec *clientcmdapi.ExecConfig) bool {
	for _, e := range exec.Env {
		if e.Name != DisableEnvVar {
			continue
		}
		disabled, err := strconv.ParseBool(e.Value)
		if err == nil && disabled {
			return true
		}
	}
	return false
}

// execEnvWithout returns env without entries named name. The marker is
// removed before identity checks so a false value is not mistaken for an
// env entry that selects a different account.
func execEnvWithout(env []clientcmdapi.ExecEnvVar, name string) []clientcmdapi.ExecEnvVar {
	if len(env) == 0 {
		return env
	}
	filtered := make([]clientcmdapi.ExecEnvVar, 0, len(env))
	for _, e := range env {
		if e.Name != name {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// selectsApplicationDefaultCredentials reports whether a
// gke-gcloud-auth-plugin stanza authenticates with exactly the Application
// Default Credentials the Meshery process uses: gkeADCFlag is set (bare or
// with a true value), no other argument is present, and the stanza sets no
// environment variables. DisableEnvVar is not such a variable: a false
// value is ignored here (a true value is rejected by Recognize before this
// runs). Any other argument (for example --impersonate_service_account or
// --use_edge_cloud) or env entry (for example GOOGLE_APPLICATION_CREDENTIALS,
// CLOUDSDK_CONFIG or HOME, which move the ADC file the plugin reads) could
// select a different identity, so such stanzas stay on the plugin.
func selectsApplicationDefaultCredentials(exec *clientcmdapi.ExecConfig) bool {
	if len(execEnvWithout(exec.Env, DisableEnvVar)) > 0 || len(exec.Args) == 0 {
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
// "aws-iam-authenticator token". A stanza that passes --forward-session-name
// stays on the plugin: the plugin forwards the caller's STS session name
// when assuming a role, and in-process minting uses --session-name or a
// fixed name instead, which would change the Kubernetes username when the
// cluster maps {{SessionName}}.
func parseAWSIAMAuthenticator(args []string) (EKSConfig, bool) {
	if !containsArg(args, "token") {
		return EKSConfig{}, false
	}
	if forwardSessionName(args) {
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

// forwardSessionName reports whether args enable --forward-session-name:
// bare, or with a true or unparseable value. The plugin's boolean flag is
// last-occurrence-wins, so every occurrence is scanned: an explicit false
// only leaves forwarding off when no later occurrence enables the flag. An
// unparseable value counts, so the stanza stays on the plugin, which
// rejects it as it would have before.
func forwardSessionName(args []string) bool {
	for _, arg := range args {
		if arg == "--forward-session-name" {
			return true
		}
		if name, value, found := strings.Cut(arg, "="); found && name == "--forward-session-name" {
			enabled, err := strconv.ParseBool(value)
			if err != nil || enabled {
				return true
			}
		}
	}
	return false
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
