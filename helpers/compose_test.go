package helpers

import (
	"context"
	"testing"

	dockerCmd "github.com/docker/cli/cli/command"
	cliconfig "github.com/docker/cli/cli/config"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/compose-cli/api/client"
	dockerconfig "github.com/docker/docker/cli/config"
	dockerClient "github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
)

func mockDockerClient() (*dockerClient.APIClient, error) {
	// Get the Docker configuration
	dockerCfg, err := cliconfig.Load(dockerconfig.Dir())
	if err != nil {
		return nil, err
	}

	//connection to docker-client
	cli, err := dockerCmd.NewAPIClientFromFlags(cliflags.NewCommonOptions(), dockerCfg)
	if err != nil {
		return nil, err
	}

	return &cli, nil
}

func TestNewComposeClient(t *testing.T) {
	// Test that NewComposeClient returns a non-nil client and no error

	dc, err := mockDockerClient()
	assert.NotNil(t, dc)
	assert.NoError(t, err)

	c := NewComposeClientFromDocker(dc)
	assert.NotNil(t, c)
}

func TestGetVersion(t *testing.T) {
	// Test that GetVersion returns a non-empty string and no error
	dc, err := mockDockerClient()
	assert.NoError(t, err)

	c := NewComposeClientFromDocker(dc)
	assert.NotNil(t, c)

	version, err := GetVersion(context.Background(), c)
	assert.NotEmpty(t, version)
	assert.NoError(t, err)
}

func TestUp(t *testing.T) {
	// Test that Up returns no error
	ctx := context.Background()
	c := &client.Client{} // Mock client

	err := Up(ctx, *c, "test_project", "docker-compose.yml", false)
	assert.NoError(t, err)
}

func TestRm(t *testing.T) {
	// Test that Rm returns no error
	ctx := context.Background()
	c := &client.Client{} // Mock client

	err := Rm(ctx, *c, "test_project", "docker-compose.yml", false)
	assert.NoError(t, err)
}

func TestStop(t *testing.T) {
	// Test that Stop returns no error
	ctx := context.Background()
	c := &client.Client{} // Mock client

	err := Stop(ctx, *c, "test_project", "docker-compose.yml", false)
	assert.NoError(t, err)
}

func TestPs(t *testing.T) {
	// Test that Ps returns no error
	ctx := context.Background()
	c := &client.Client{} // Mock client

	err := Ps(ctx, c, false, false, "table")
	assert.NoError(t, err)
}

func TestListContainers(t *testing.T) {
	// Test that ListContainers returns a non-nil slice of ContainerView and no error
	ctx := context.Background()
	c := &client.Client{} // Mock client

	containers, err := ListContainers(ctx, c, false)
	assert.NotNil(t, containers)
	assert.NoError(t, err)
}

func TestPull(t *testing.T) {
	// Test that Pull returns no error
	ctx := context.Background()
	c := &client.Client{} // Mock client

	err := Pull(ctx, c, "docker-compose.yml", "test_project")
	assert.NoError(t, err)
}

func TestGetLogs(t *testing.T) {
	// Test that GetLogs returns no error
	ctx := context.Background()
	c := &client.Client{} // Mock client

	err := GetLogs(ctx, c, "docker-compose.yml", "100", false)
	assert.NoError(t, err)
}
