package helpers

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"github.com/docker/compose-cli/api/client"

	"github.com/docker/compose-cli/local"

	"github.com/docker/compose-cli/api/containers"
	"github.com/docker/compose-cli/utils/formatter"

	dockerClient "github.com/docker/docker/client"

	dockerCmd "github.com/docker/cli/cli/command"
	formatter2 "github.com/docker/cli/cli/command/formatter"
	cliconfig "github.com/docker/cli/cli/config"
	cliflags "github.com/docker/cli/cli/flags"
	dockerconfig "github.com/docker/docker/cli/config"

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

func NewDockerAPIClientFromConfig(configDir string) (*dockerClient.APIClient, error) {
	if configDir == "" {
		configDir = dockerconfig.Dir()
	}

	// Get the Docker configuration
	dockerCfg, err := cliconfig.Load(configDir)
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

func Ps(ctx context.Context, c *client.Client, all, quiet bool, formatOpt, composeFilePath string) error {
	project, err := projectFromComposeFilePath(composeFilePath)
	if err != nil {
		return err
	}

	containerList, err := c.ComposeService().Ps(ctx, project.Name, api.PsOptions{All: all})
	if err != nil {
		return fmt.Errorf("failed to fetch containers: %w", err)
	}

	if quiet {
		for _, c := range containerList {
			fmt.Println(c.ID)
		}
		return nil
	}
	return format.Print(containerList, formatOpt, os.Stdout,
		writer(containerList),
		"NAME", "IMAGE", "COMMAND", "SERVICE", "CREATED", "STATUS", "PORTS")
}

func writer(containers []api.ContainerSummary) func(w io.Writer) {
	return func(w io.Writer) {
		for _, container := range containers {
			ports := displayablePorts(container)
			status := container.State
			if status == "running" && container.Health != "" {
				status = fmt.Sprintf("%s (%s)", container.State, container.Health)
			} else if status == "exited" || status == "dead" {
				status = fmt.Sprintf("%s (%d)", container.State, container.ExitCode)
			}
			command := formatter2.Ellipsis(container.Command, 20)
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", container.Name, strconv.Quote(command), container.Service, status, ports)
		}
	}
}

type portRange struct {
	pStart   int
	pEnd     int
	tStart   int
	tEnd     int
	IP       string
	protocol string
}

func (pr portRange) String() string {
	var (
		pub string
		tgt string
	)

	if pr.pEnd > pr.pStart {
		pub = fmt.Sprintf("%s:%d-%d->", pr.IP, pr.pStart, pr.pEnd)
	} else if pr.pStart > 0 {
		pub = fmt.Sprintf("%s:%d->", pr.IP, pr.pStart)
	}
	if pr.tEnd > pr.tStart {
		tgt = fmt.Sprintf("%d-%d", pr.tStart, pr.tEnd)
	} else {
		tgt = fmt.Sprintf("%d", pr.tStart)
	}
	return fmt.Sprintf("%s%s/%s", pub, tgt, pr.protocol)
}

// displayablePorts is copy pasted from https://github.com/docker/compose/blob/7c0e865960fa595174bfc39c9c7af7f56d5a2b2f/cmd/compose/ps.go#L211
func displayablePorts(c api.ContainerSummary) string {
	if c.Publishers == nil {
		return ""
	}

	sort.Sort(c.Publishers)

	pr := portRange{}
	ports := []string{}
	for _, p := range c.Publishers {
		prIsRange := pr.tEnd != pr.tStart
		tOverlaps := p.TargetPort <= pr.tEnd

		// Start a new port-range if:
		// - the protocol is different from the current port-range
		// - published or target port are not consecutive to the current port-range
		// - the current port-range is a _range_, and the target port overlaps with the current range's target-ports
		if p.Protocol != pr.protocol || p.URL != pr.IP || p.PublishedPort-pr.pEnd > 1 || p.TargetPort-pr.tEnd > 1 || prIsRange && tOverlaps {
			// start a new port-range, and print the previous port-range (if any)
			if pr.pStart > 0 {
				ports = append(ports, pr.String())
			}
			pr = portRange{
				pStart:   p.PublishedPort,
				pEnd:     p.PublishedPort,
				tStart:   p.TargetPort,
				tEnd:     p.TargetPort,
				protocol: p.Protocol,
				IP:       p.URL,
			}
			continue
		}
		pr.pEnd = p.PublishedPort
		pr.tEnd = p.TargetPort
	}
	if pr.tStart > 0 {
		ports = append(ports, pr.String())
	}
	return strings.Join(ports, ", ")
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

func Pull(ctx context.Context, c *client.Client, composeFilePath string) error {
	project, err := projectFromComposeFilePath(composeFilePath)
	if err != nil {
		return err
	}

	return c.ComposeService().Pull(ctx, project, api.PullOptions{})
}

func GetLogs(ctx context.Context, c *client.Client, composeFilePath, tail string, follow bool, logConsumer api.LogConsumer) error {
	project, err := projectFromComposeFilePath(composeFilePath)
	if err != nil {
		return err
	}

	return c.ComposeService().Logs(ctx, project.Name, logConsumer, api.LogOptions{Tail: tail, Follow: follow})
}
