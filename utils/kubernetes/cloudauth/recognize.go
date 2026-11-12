package cloudauth

import (
	"fmt"
	"path/filepath"
	"strings"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Provider identifies a recognized cloud Kubernetes authentication mechanism.
type Provider string

const (
	// ProviderUnknown indicates an unrecognized or generic authentication mechanism
	// that must continue using client-go's default loader/exec behavior.
	ProviderUnknown Provider = ""
	// ProviderEKS indicates Amazon EKS authentication via STS GetCallerIdentity.
	ProviderEKS Provider = "eks"
	// ProviderGKE indicates Google GKE authentication via Google OAuth2 tokens.
	ProviderGKE Provider = "gke"
)

// MatchedConfig holds parsed parameters extracted from a recognized kubeconfig
// exec or auth-provider stanza.
type MatchedConfig struct {
	Provider        Provider
	Command         string
	ClusterName     string
	Region          string
	RoleARN         string
	ExternalID      string
	RoleSessionName string
	Profile         string
	Endpoint        string
	Env             map[string]string
	LegacyProvider  bool
}

// String redacts potentially sensitive environment variables when formatting.
func (m MatchedConfig) String() string {
	return fmt.Sprintf("cloudauth.MatchedConfig{Provider:%s, Command:%s, ClusterName:%s, Region:%s}",
		m.Provider, m.Command, m.ClusterName, m.Region)
}

// GoString redacts potentially sensitive environment variables when formatting.
func (m MatchedConfig) GoString() string {
	return m.String()
}

// MatchAuthInfo inspects a kubeconfig AuthInfo and determines whether it matches
// a supported in-process EKS or GKE authentication configuration.
//
// When matched == false and err == nil, callers must fall back to the existing
// client-go kubeconfig loader (#986 behavior).
func MatchAuthInfo(authInfo *clientcmdapi.AuthInfo) (MatchedConfig, bool, error) {
	if authInfo == nil {
		return MatchedConfig{}, false, nil
	}

	if authInfo.Exec != nil {
		return matchExecConfig(authInfo.Exec)
	}

	if authInfo.AuthProvider != nil && strings.EqualFold(strings.TrimSpace(authInfo.AuthProvider.Name), "gcp") {
		return MatchedConfig{
			Provider:       ProviderGKE,
			Command:        "auth-provider:gcp",
			LegacyProvider: true,
		}, true, nil
	}

	return MatchedConfig{}, false, nil
}

func matchExecConfig(execCfg *clientcmdapi.ExecConfig) (MatchedConfig, bool, error) {
	cmdName := normalizeCommandName(execCfg.Command)
	envMap := execEnvToMap(execCfg.Env)

	switch cmdName {
	case "aws":
		return matchAWSCLIExec(cmdName, execCfg.Args, envMap)
	case "aws-iam-authenticator":
		return matchAWSIAMAuthenticatorExec(cmdName, execCfg.Args, envMap)
	case "gke-gcloud-auth-plugin":
		return matchGKEPluginExec(cmdName, execCfg.Args, envMap)
	case "gcloud":
		return matchGCloudExec(cmdName, execCfg.Args, envMap)
	default:
		return MatchedConfig{}, false, nil
	}
}

func normalizeCommandName(command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return ""
	}
	base := filepath.Base(trimmed)
	// Handle Windows-style paths even on POSIX hosts.
	if idx := strings.LastIndex(base, `\`); idx >= 0 {
		base = base[idx+1:]
	}
	lower := strings.ToLower(base)
	for _, ext := range []string{".exe", ".cmd", ".bat"} {
		lower = strings.TrimSuffix(lower, ext)
	}
	return lower
}

func execEnvToMap(vars []clientcmdapi.ExecEnvVar) map[string]string {
	if len(vars) == 0 {
		return nil
	}
	out := make(map[string]string, len(vars))
	for _, v := range vars {
		if v.Name != "" {
			out[v.Name] = v.Value
		}
	}
	return out
}

// awsGlobalValueFlags are AWS CLI global options that consume the following arg.
var awsGlobalValueFlags = map[string]bool{
	"--profile":             true,
	"--region":              true,
	"--output":              true,
	"--query":               true,
	"--endpoint-url":        true,
	"--ca-bundle":           true,
	"--cli-read-timeout":    true,
	"--cli-connect-timeout": true,
	"--color":               true,
	"--cli-input-json":      true,
	"--cli-input-yaml":      true,
	"-r":                    true,
	"-o":                    true,
}

func matchAWSCLIExec(cmdName string, args []string, env map[string]string) (MatchedConfig, bool, error) {
	positionals := make([]string, 0, 2)
	matched := MatchedConfig{
		Provider: ProviderEKS,
		Command:  cmdName,
		Env:      env,
		Region:   firstNonEmpty(env["AWS_REGION"], env["AWS_DEFAULT_REGION"]),
		Profile:  env["AWS_PROFILE"],
	}

	var clusterIDFlag string
	var hasSSOFlag, hasMFAFlag bool

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}

		name, val, hasEq := splitFlag(arg)
		switch name {
		case "--cluster-name":
			val = nextFlagValue(args, &i, val, hasEq)
			matched.ClusterName = val
		case "--cluster-id":
			val = nextFlagValue(args, &i, val, hasEq)
			clusterIDFlag = val
		case "--region", "-r":
			val = nextFlagValue(args, &i, val, hasEq)
			if val != "" {
				matched.Region = val
			}
		case "--role-arn":
			val = nextFlagValue(args, &i, val, hasEq)
			matched.RoleARN = val
		case "--profile":
			val = nextFlagValue(args, &i, val, hasEq)
			if val != "" {
				matched.Profile = val
			}
		case "--endpoint-url":
			val = nextFlagValue(args, &i, val, hasEq)
			matched.Endpoint = val
		case "--sso":
			hasSSOFlag = true
		case "--mfa-serial", "--token-code":
			hasMFAFlag = true
			if !hasEq && i+1 < len(args) {
				i++
			}
		default:
			if !hasEq && awsGlobalValueFlags[name] && i+1 < len(args) {
				i++
			}
		}
	}

	if clusterIDFlag != "" {
		matched.ClusterName = clusterIDFlag
	}
	if len(positionals) < 2 || positionals[0] != "eks" || positionals[1] != "get-token" {
		return MatchedConfig{}, false, nil
	}
	if hasSSOFlag || hasMFAFlag || env["AWS_MFA_SERIAL"] != "" {
		return matched, true, ErrUnsupportedCloudAuthFlow("AWS SSO and MFA flags in EKS exec config are not supported in v1")
	}
	if strings.TrimSpace(matched.ClusterName) == "" {
		return matched, true, ErrInvalidCloudExecConfig(fmt.Errorf("`aws eks get-token` exec config is missing --cluster-name or --cluster-id"))
	}
	return matched, true, nil
}

func matchAWSIAMAuthenticatorExec(cmdName string, args []string, env map[string]string) (MatchedConfig, bool, error) {
	positionals := make([]string, 0, 1)
	matched := MatchedConfig{
		Provider: ProviderEKS,
		Command:  cmdName,
		Env:      env,
		Region:   firstNonEmpty(env["AWS_REGION"], env["AWS_DEFAULT_REGION"]),
		Profile:  env["AWS_PROFILE"],
	}

	var hasMFA bool
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}

		name, val, hasEq := splitFlag(arg)
		switch name {
		case "-i", "--cluster-id":
			val = nextFlagValue(args, &i, val, hasEq)
			matched.ClusterName = val
		case "-r", "--role":
			val = nextFlagValue(args, &i, val, hasEq)
			matched.RoleARN = val
		case "-e", "--external-id":
			val = nextFlagValue(args, &i, val, hasEq)
			matched.ExternalID = val
		case "-s", "--session-name":
			val = nextFlagValue(args, &i, val, hasEq)
			matched.RoleSessionName = val
		case "--region":
			val = nextFlagValue(args, &i, val, hasEq)
			if val != "" {
				matched.Region = val
			}
		case "--profile":
			val = nextFlagValue(args, &i, val, hasEq)
			if val != "" {
				matched.Profile = val
			}
		case "--token", "--mfa-serial":
			hasMFA = true
			if !hasEq && i+1 < len(args) {
				i++
			}
		}
	}

	if len(positionals) == 0 || positionals[0] != "token" {
		return MatchedConfig{}, false, nil
	}
	if hasMFA || env["AWS_MFA_SERIAL"] != "" {
		return matched, true, ErrUnsupportedCloudAuthFlow("MFA prompts in aws-iam-authenticator exec config are not supported in v1")
	}
	if strings.TrimSpace(matched.ClusterName) == "" {
		return matched, true, ErrInvalidCloudExecConfig(fmt.Errorf("`aws-iam-authenticator token` exec config is missing -i / --cluster-id"))
	}
	return matched, true, nil
}

func matchGKEPluginExec(cmdName string, args []string, env map[string]string) (MatchedConfig, bool, error) {
	matched := MatchedConfig{
		Provider: ProviderGKE,
		Command:  cmdName,
		Env:      env,
	}
	for i := 0; i < len(args); i++ {
		name, _, _ := splitFlag(args[i])
		if name == "--impersonate_service_account" || name == "--impersonate-service-account" {
			return matched, true, ErrUnsupportedCloudAuthFlow("service-account impersonation in gke-gcloud-auth-plugin is not supported in v1")
		}
	}

	return matched, true, nil
}

var gcloudValueFlags = map[string]bool{
	"--format":          true,
	"--project":         true,
	"--account":         true,
	"--configuration":   true,
	"--billing-project": true,
	"--verbosity":       true,
	"--filter":          true,
	"--zone":            true,
	"--region":          true,
}

func matchGCloudExec(cmdName string, args []string, env map[string]string) (MatchedConfig, bool, error) {
	positionals := make([]string, 0, 3)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}
		name, _, hasEq := splitFlag(arg)
		if name == "--impersonate-service-account" || name == "--impersonate_service_account" {
			return MatchedConfig{
				Provider: ProviderGKE,
				Command:  cmdName,
				Env:      env,
			}, true, ErrUnsupportedCloudAuthFlow("service-account impersonation in gcloud exec config is not supported in v1")
		}
		if !hasEq && gcloudValueFlags[name] && i+1 < len(args) {
			i++
		}
	}

	isTokenHelper := len(positionals) == 0 ||
		(len(positionals) >= 2 && positionals[0] == "config" && positionals[1] == "config-helper") ||
		(len(positionals) >= 2 && positionals[0] == "auth" && (positionals[1] == "print-access-token" || positionals[1] == "application-default")) ||
		(len(positionals) >= 2 && positionals[0] == "container" && positionals[1] == "clusters")

	if !isTokenHelper {
		return MatchedConfig{}, false, nil
	}

	return MatchedConfig{
		Provider: ProviderGKE,
		Command:  cmdName,
		Env:      env,
	}, true, nil
}

func splitFlag(arg string) (name, value string, hasEq bool) {
	if idx := strings.IndexByte(arg, '='); idx >= 0 {
		return arg[:idx], arg[idx+1:], true
	}
	return arg, "", false
}

func nextFlagValue(args []string, idx *int, inlineVal string, hasEq bool) string {
	if hasEq {
		return inlineVal
	}
	if *idx+1 < len(args) {
		*idx++
		return args[*idx]
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
