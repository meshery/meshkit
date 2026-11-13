package kubernetes

import (
	"fmt"
	"strconv"

	"github.com/meshery/meshkit/utils/kubernetes/cloudauth"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// SetInProcessCloudAuthForContext sets the opt-out for in-process cloud token
// minting on contextName's user. mesheryctl and Meshery Server use it to turn
// minting off for a kubeconfig context. The marker is stored on the
// context's user (AuthInfo) entry, not on the context, so every context that
// shares that user switches together; contexts with other users are not
// affected.
//
// enabled false writes cloudauth.DisableEnvVar (MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH)
// with the value "true" on the context user's exec stanza, so the
// Authenticator leaves that stanza on the exec plugin. enabled true removes
// that env entry. The process-wide variable of the same name is a separate
// switch and is not read or written here.
//
// It returns an error when config is nil, the context is missing, or the
// context's user is missing. A user with no exec stanza is left unchanged
// and returns nil: there is no plugin to leave in place.
func SetInProcessCloudAuthForContext(config *clientcmdapi.Config, contextName string, enabled bool) error {
	auth, err := contextAuthInfo(config, contextName)
	if err != nil {
		return err
	}
	if auth.Exec == nil {
		return nil
	}
	auth.Exec.Env = setInProcessCloudAuthEnv(auth.Exec.Env, enabled)
	return nil
}

// InProcessCloudAuthEnabledForContext reports whether in-process cloud token
// minting is enabled for contextName's user. It is the read side of
// SetInProcessCloudAuthForContext for mesheryctl and Meshery Server. It reads
// the marker on the context's user (AuthInfo) entry, so every context that
// shares that user reports the same value.
//
// It returns false when config is nil, the context is missing, or the
// context's user is missing, and when the user's exec stanza sets
// MESHKIT_DISABLE_INPROCESS_CLOUD_AUTH to a value strconv.ParseBool accepts
// as true. It returns true otherwise, including when the user has no exec
// stanza and when the env entry is present with a false value. It does not
// consult the process-wide variable of the same name.
func InProcessCloudAuthEnabledForContext(config *clientcmdapi.Config, contextName string) bool {
	auth, err := contextAuthInfo(config, contextName)
	if err != nil {
		return false
	}
	if auth.Exec == nil {
		return true
	}
	for _, env := range auth.Exec.Env {
		if env.Name != cloudauth.DisableEnvVar {
			continue
		}
		disabled, parseErr := strconv.ParseBool(env.Value)
		if parseErr == nil && disabled {
			return false
		}
	}
	return true
}

func contextAuthInfo(config *clientcmdapi.Config, contextName string) (*clientcmdapi.AuthInfo, error) {
	if config == nil {
		return nil, fmt.Errorf("kubeconfig is nil")
	}
	ctx := config.Contexts[contextName]
	if ctx == nil {
		return nil, fmt.Errorf("kubeconfig context %q not found", contextName)
	}
	auth := config.AuthInfos[ctx.AuthInfo]
	if auth == nil {
		return nil, fmt.Errorf("kubeconfig user %q for context %q not found", ctx.AuthInfo, contextName)
	}
	return auth, nil
}

// setInProcessCloudAuthEnv drops every existing opt-out entry. When enabled
// is false it appends one true entry. Other env entries are preserved.
func setInProcessCloudAuthEnv(env []clientcmdapi.ExecEnvVar, enabled bool) []clientcmdapi.ExecEnvVar {
	filtered := make([]clientcmdapi.ExecEnvVar, 0, len(env)+1)
	for _, entry := range env {
		if entry.Name == cloudauth.DisableEnvVar {
			continue
		}
		filtered = append(filtered, entry)
	}
	if !enabled {
		filtered = append(filtered, clientcmdapi.ExecEnvVar{Name: cloudauth.DisableEnvVar, Value: "true"})
	}
	return filtered
}
