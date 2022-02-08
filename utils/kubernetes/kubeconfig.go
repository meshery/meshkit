package kubernetes

import (
	"os"
	"path/filepath"

	"github.com/layer5io/meshkit/models"
	"github.com/layer5io/meshkit/utils"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

func (c *Client) GetKubeConfig() (*models.Kubeconfig, error) {
	// Look for kubeconfig from the path mentioned in $KUBECONFIG
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(utils.GetHome(), ".kube", "config")
	}

	var config *models.Kubeconfig
	file, err := os.ReadFile(kubeconfig)
	if err != nil {
		err = errors.Wrap(err, "could not read kube config:")
		return nil, err
	}
	if err := yaml.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Client) GetCurrentContext() (string, error) {
	config, err := c.GetKubeConfig()
	if err != nil {
		return "", err
	}

	if config.CurrentContext == "" {
		return "", errors.New("current context cluster not found or empty")
	}

	return config.CurrentContext, nil
}
