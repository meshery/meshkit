package models

import "time"

type Kubeconfig struct {
	APIVersion string `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
	Clusters   []struct {
		Cluster struct {
			CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty" json:"certificate-authority-data,omitempty"`
			Server                   string `yaml:"server,omitempty" json:"server,omitempty"`
			InsecureSkipTLSVerify    *bool  `yaml:"insecure-skip-tls-verify,omitempty" json:"insecure-skip-tls-verify,omitempty"`
		} `yaml:"cluster,omitempty" json:"cluster,omitempty"`
		Name string `yaml:"name,omitempty" json:"name,omitempty"`
	} `yaml:"clusters,omitempty" json:"clusters,omitempty"`
	Contexts []struct {
		Context struct {
			Cluster   string `yaml:"cluster,omitempty" json:"cluster,omitempty"`
			Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
			User      string `yaml:"user,omitempty" json:"user,omitempty"`
		} `yaml:"context,omitempty" json:"context,omitempty"`
		Name string `yaml:"name,omitempty" json:"name,omitempty"`
	} `yaml:"contexts,omitempty" json:"contexts,omitempty"`
	CurrentContext string `yaml:"current-context,omitempty" json:"current-context,omitempty"`
	Kind           string `yaml:"kind,omitempty" json:"kind,omitempty"`
	Preferences    struct {
	} `yaml:"preferences,omitempty" json:"preferences,omitempty"`
	Users []struct {
		Name string `yaml:"name,omitempty" json:"name,omitempty"`
		User struct {
			Exec struct {
				APIVersion string   `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
				Args       []string `yaml:"args,omitempty" json:"args,omitempty"`
				Command    string   `yaml:"command,omitempty" json:"command,omitempty"`
				Env        []struct {
					Name  string `yaml:"name,omitempty" json:"name,omitempty"`
					Value string `yaml:"value,omitempty" json:"value,omitempty"`
				} `yaml:"env,omitempty" json:"env,omitempty"`
			} `yaml:"exec,omitempty" json:"exec,omitempty"`
			AuthProvider struct {
				Config struct {
					AccessToken string    `yaml:"access-token,omitempty" json:"access-token,omitempty"`
					CmdArgs     string    `yaml:"cmd-args,omitempty" json:"cmd-args,omitempty"`
					CmdPath     string    `yaml:"cmd-path,omitempty" json:"cmd-path,omitempty"`
					Expiry      time.Time `yaml:"expiry,omitempty" json:"expiry,omitempty"`
					ExpiryKey   string    `yaml:"expiry-key,omitempty" json:"expiry-key,omitempty"`
					TokenKey    string    `yaml:"token-key,omitempty" json:"token-key,omitempty"`
				} `yaml:"config,omitempty" json:"config,omitempty"`
				Name string `yaml:"name,omitempty" json:"name,omitempty"`
			} `yaml:"auth-provider,omitempty" json:"auth-provider,omitempty"`
			ClientCertificateData string `yaml:"client-certificate-data,omitempty" json:"client-certificate-data,omitempty"`
			ClientKeyData         string `yaml:"client-key-data,omitempty" json:"client-key-data,omitempty"`
			Token                 string `yaml:"token,omitempty" json:"token,omitempty"`
		} `yaml:"user,omitempty" json:"user,omitempty"`
	} `yaml:"users,omitempty" json:"users,omitempty"`
}
