package models

import "time"

// Kubeconfig is structure of the kubeconfig file
type Kubeconfig struct {
	APIVersion string `yaml:"apiVersion,omitempty"`
	Clusters   []struct {
		Cluster struct {
			CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
			Server                   string `yaml:"server,omitempty"`
		} `yaml:"cluster,omitempty"`
		Name string `yaml:"name,omitempty"`
	} `yaml:"clusters,omitempty"`
	Contexts []struct {
		Context struct {
			Cluster   string `yaml:"cluster,omitempty"`
			Namespace string `yaml:"namespace,omitempty"`
			User      string `yaml:"user,omitempty"`
		} `yaml:"context,omitempty"`
		Name string `yaml:"name,omitempty"`
	} `yaml:"contexts,omitempty"`
	CurrentContext string `yaml:"current-context,omitempty"`
	Kind           string `yaml:"kind,omitempty"`
	Preferences    struct {
	} `yaml:"preferences,omitempty"`
	Users []struct {
		Name string `yaml:"name,omitempty"`
		User struct {
			Exec struct {
				APIVersion string   `yaml:"apiVersion,omitempty"`
				Args       []string `yaml:"args,omitempty"`
				Command    string   `yaml:"command,omitempty"`
				Env        []struct {
					Name  string `yaml:"name,omitempty"`
					Value string `yaml:"value,omitempty"`
				} `yaml:"env,omitempty"`
			} `yaml:"exec,omitempty"`
			AuthProvider struct {
				Config struct {
					AccessToken string    `yaml:"access-token,omitempty"`
					CmdArgs     string    `yaml:"cmd-args,omitempty"`
					CmdPath     string    `yaml:"cmd-path,omitempty"`
					Expiry      time.Time `yaml:"expiry,omitempty"`
					ExpiryKey   string    `yaml:"expiry-key,omitempty"`
					TokenKey    string    `yaml:"token-key,omitempty"`
				} `yaml:"config,omitempty"`
				Name string `yaml:"name,omitempty"`
			} `yaml:"auth-provider,omitempty"`
			ClientCertificateData string `yaml:"client-certificate-data,omitempty"`
			ClientKeyData         string `yaml:"client-key-data,omitempty"`
			Token                 string `yaml:"token,omitempty"`
		} `yaml:"user,omitempty,omitempty"`
	} `yaml:"users,omitempty"`
}
