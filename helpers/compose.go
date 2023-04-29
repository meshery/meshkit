package helpers

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"github.com/containerd/console"
	"github.com/docker/compose-cli/api/client"

	"github.com/docker/compose-cli/local"

	"github.com/docker/compose-cli/api/containers"
	"github.com/docker/compose-cli/utils/formatter"

	dockerClient "github.com/docker/docker/client"

	format "github.com/docker/compose/v2/cmd/formatter"
	"github.com/docker/compose/v2/pkg/api"
)

type ContainerView struct {
	ID      string
	Image   string
	Status  string
	Command string
	Ports   []string
}

func NewComposeClientFromDocker(c *dockerClient.APIClient) *client.Client {
	composeClient := client.NewClient("moby", local.NewService(*c))
	return &composeClient
}

// func GetVersion(ctx context.Context, c *client.Client) (string, error) {
// 	versionResult, err := mobycli.ExecSilent(ctx)
// 	if versionResult == nil {
// 		return "", err
// 	}
// 	return string(versionResult), err
// }

func projectFromComposeFilePath(composeFilePath string) (*types.Project, error) {
	// Load the Compose YAML file into memory
	yamlData, err := os.ReadFile(composeFilePath)
	if err != nil {
		panic(err)
	}

	// Create Project object from YAML
	return loader.Load(types.ConfigDetails{
		WorkingDir: filepath.Dir(composeFilePath),
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Base(composeFilePath),
			Content:  yamlData,
		}},
		Environment: nil,
	}, func(options *loader.Options) {})
}

func Up(ctx context.Context, c client.Client, composeFilePath string, detach bool) error {
	project, err := projectFromComposeFilePath(composeFilePath)
	if err != nil {
		return err
	}

	var logConsumer api.LogConsumer
	if detach {
		_, pipeWriter := io.Pipe()
		logConsumer = format.NewLogConsumer(ctx, pipeWriter, false, false)
	}

	return c.ComposeService().Up(ctx, project, api.UpOptions{Start: api.StartOptions{Attach: logConsumer}})
}

func Rm(ctx context.Context, c client.Client, composeFilePath string, force bool) error {
	project, err := projectFromComposeFilePath(composeFilePath)
	if err != nil {
		return err
	}

	return c.ComposeService().Remove(ctx, project, api.RemoveOptions{Force: force})
}

func Stop(ctx context.Context, c client.Client, composeFilePath string, force bool) error {
	project, err := projectFromComposeFilePath(composeFilePath)
	if err != nil {
		return err
	}

	return c.ComposeService().Stop(ctx, project, api.StopOptions{})
}

func Ps(ctx context.Context, c *client.Client, all, quiet bool, formatOpt string) error {
	containerList, err := c.ContainerService().List(ctx, all)
	if err != nil {
		return fmt.Errorf("failed to fetch containers: %w", err)
	}

	if quiet {
		for _, c := range containerList {
			fmt.Println(c.ID)
		}
		return nil
	}

	view := getViewFromContainerList(containerList)
	return format.Print(view, formatOpt, os.Stdout, func(w io.Writer) {
		for _, c := range view {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", c.ID, c.Image, c.Command, c.Status,
				strings.Join(c.Ports, ", "))
		}
	}, "CONTAINER ID", "IMAGE", "COMMAND", "STATUS", "PORTS")
}

func ListContainers(ctx context.Context, c *client.Client, all bool) ([]ContainerView, error) {
	containerList, err := c.ContainerService().List(ctx, all)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch containers: %w", err)
	}

	view := getViewFromContainerList(containerList)
	return view, nil
}

func getViewFromContainerList(containerList []containers.Container) []ContainerView {
	retList := make([]ContainerView, len(containerList))
	for i, c := range containerList {
		retList[i] = ContainerView{
			ID:      c.ID,
			Image:   c.Image,
			Status:  c.Status,
			Command: c.Command,
			Ports:   formatter.PortsToStrings(c.Ports, getFQDN(c)),
		}
	}
	return retList
}

func getFQDN(container containers.Container) string {
	fqdn := ""
	if container.Config != nil {
		fqdn = container.Config.FQDN
	}
	return fqdn
}

// func Pull(ctx context.Context, c *client.Client, composeFilePath, projectName string) error {
// 	if projectName == "" {
// 		projectName = "meshery"
// 	}

// 	project := types.Project{
// 		Name:         projectName,
// 		WorkingDir:   filepath.Dir(composeFilePath),
// 		ComposeFiles: []string{composeFilePath},
// 	}

// 	err := c.ComposeService().Pull(ctx, &project, api.PullOptions{})
// 	if err != nil {
// 		return fmt.Errorf("failed to pull images: %w", err)
// 	}

// 	return nil
// }

func GetLogs(ctx context.Context, c *client.Client, composeFilePath, tail string, follow bool) error {
	req := containers.LogsRequest{
		Follow: follow,
		Tail:   tail,
	}

	var con io.Writer = os.Stdout
	if cff, err := console.ConsoleFromFile(os.Stdout); err == nil {
		size, err := cff.Size()
		if err != nil {
			return err
		}
		req.Width = int(size.Width)
		con = cff
	}

	req.Writer = con

	return c.ContainerService().Logs(ctx, composeFilePath, req)
}
