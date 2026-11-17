package kubernetes

import (
	"os"
	"path/filepath"

	"github.com/meshery/meshkit/models"
	"github.com/meshery/meshkit/utils"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func (c *Client) GetKubeConfig() (*models.Kubeconfig, error) {
	// Look for kubeconfig from the path mentioned in $KUBECONFIG
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(utils.GetHome(), ".kube", "config")
	}

	file, err := os.ReadFile(kubeconfig)
	if err != nil {
		err = errors.Wrap(err, "could not read kubeconfig:")
		return nil, err
	}
	// Decode into a value, not a pointer: an empty file, or one holding only a comment
	// or null, decodes without error, which would leave a pointer nil and return
	// (nil, nil) for callers to dereference.
	var config models.Kubeconfig
	if err := yaml.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Client) GetCurrentContext() (string, error) {
	config, err := c.GetKubeConfig()
	if err != nil {
		return "", err
	}

	return config.CurrentContext, nil
}
