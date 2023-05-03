package helpers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	format "github.com/docker/compose/v2/cmd/formatter"
	"github.com/stretchr/testify/assert"
)

var testDockerComposeConfig string = "./docker-compose.test.yml"

func TestNewDockerAPIClientFromConfig(t *testing.T) {
	dc, err := NewDockerAPIClientFromConfig("")

	assert.NotNil(t, dc)
	assert.NoError(t, err)
}

func TestNewComposeClient(t *testing.T) {
	// Test that NewComposeClient returns a non-nil client and no error

	dc, err := NewDockerAPIClientFromConfig("")
	assert.NotNil(t, dc)
	assert.NoError(t, err)

	c := NewComposeClientFromDocker(dc)
	assert.NotNil(t, c)
}

// func TestGetVersion(t *testing.T) {
// 	// Test that GetVersion returns a non-empty string and no error
// 	dc, err := mockDockerClient()
// 	assert.NoError(t, err)

// 	c := NewComposeClientFromDocker(dc)
// 	assert.NotNil(t, c)

// 	version, err := GetVersion(context.Background(), c)
// 	assert.NotEmpty(t, version)
// 	assert.NoError(t, err)
// }

func TestOrchestration(t *testing.T) {
	t.Run("Test Up command", TestUp)

	t.Run("Test Stop command", TestStop)

	t.Run("Test Compose command", TestStop)
}

func TestUp(t *testing.T) {
	// Test that Up returns no error
	ctx := context.Background()
	dc, err := NewDockerAPIClientFromConfig("")
	assert.NoError(t, err)

	c := NewComposeClientFromDocker(dc)
	assert.NotNil(t, c)

	composeFilePath, err := filepath.Abs(testDockerComposeConfig)
	assert.NoError(t, err)

	err = Up(ctx, *c, composeFilePath, false)
	assert.NoError(t, err)
}

func TestRm(t *testing.T) {
	// Test that Rm returns no error
	ctx := context.Background()
	dc, err := NewDockerAPIClientFromConfig("")
	assert.NoError(t, err)

	c := NewComposeClientFromDocker(dc)
	assert.NotNil(t, c)

	err = Rm(ctx, *c, testDockerComposeConfig, false)
	assert.NoError(t, err)
}

func TestStop(t *testing.T) {
	// Test that Stop returns no error
	ctx := context.Background()
	dc, err := NewDockerAPIClientFromConfig("")
	assert.NoError(t, err)

	c := NewComposeClientFromDocker(dc)
	assert.NotNil(t, c)

	err = Stop(ctx, *c, "docker-compose.test.yml", false)
	assert.NoError(t, err)
}

func TestPs(t *testing.T) {
	// Test that Ps returns no error
	ctx := context.Background()
	dc, err := NewDockerAPIClientFromConfig("")
	assert.NoError(t, err)

	c := NewComposeClientFromDocker(dc)
	assert.NotNil(t, c)

	err = Ps(ctx, c, false, false, "")
	assert.NoError(t, err)
}

func TestListContainers(t *testing.T) {
	// Test that ListContainers returns a non-nil slice of ContainerView and no error
	ctx := context.Background()
	dc, err := NewDockerAPIClientFromConfig("")
	assert.NoError(t, err)

	c := NewComposeClientFromDocker(dc)
	assert.NotNil(t, c)

	containers, err := ListContainers(ctx, c, false)
	assert.NotNil(t, containers)
	assert.NoError(t, err)
}

func TestPull(t *testing.T) {
	// Test that Pull returns no error
	ctx := context.Background()
	dc, err := NewDockerAPIClientFromConfig("")
	assert.NoError(t, err)

	c := NewComposeClientFromDocker(dc)
	assert.NotNil(t, c)

	err = Pull(ctx, c, testDockerComposeConfig)
	assert.NoError(t, err)
}

func TestGetLogs(t *testing.T) {
	// Test that GetLogs returns no error
	ctx := context.Background()
	dc, err := NewDockerAPIClientFromConfig("")
	assert.NoError(t, err)

	c := NewComposeClientFromDocker(dc)
	assert.NotNil(t, c)

	logConsumer := format.NewLogConsumer(ctx, os.Stdout, false, false)
	err = GetLogs(ctx, c, testDockerComposeConfig, "10", false, logConsumer)
	assert.NoError(t, err)
}
