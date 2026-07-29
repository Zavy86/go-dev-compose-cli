package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/moby/moby/client"
)

type Client struct {
	cli         *client.Client
	composePath string
	projectName string
}

func NewClient(composePath string, projectName string) (*Client, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &Client{
		cli:         cli,
		composePath: composePath,
		projectName: projectName,
	}, nil
}

func (d *Client) ProjectName() string {
	return d.projectName
}

func demuxMobyStream(src io.Reader, dst io.Writer) error {
	header := make([]byte, 8)
	for {
		_, err := io.ReadFull(src, header)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		count := int(header[4])<<24 | int(header[5])<<16 | int(header[6])<<8 | int(header[7])

		_, err = io.CopyN(dst, src, int64(count))
		if err != nil {
			return err
		}
	}
}

func (d *Client) StreamLogs(ctx context.Context, serviceName string, writer io.Writer) error {
	containerID, err := d.findContainerID(ctx, serviceName)
	if err != nil {
		return err
	}

	options := client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "100",
	}

	out, err := d.cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return err
	}
	defer out.Close()

	return demuxMobyStream(out, writer)
}

func (d *Client) StreamAllLogs(ctx context.Context, writer io.Writer) error {
	res, err := d.cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return err
	}

	type target struct {
		id   string
		name string
	}
	var targets []target

	for _, c := range res.Items {
		proj := c.Labels["com.docker.compose.project"]
		svc := c.Labels["com.docker.compose.service"]

		if proj == d.projectName && svc != "" {
			targets = append(targets, target{id: c.ID, name: svc})
		}
	}

	if len(targets) == 0 {
		return fmt.Errorf("no container found for project %s", d.projectName)
	}

	for _, t := range targets {
		go func(containerID, serviceName string) {
			options := client.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Follow:     true,
				Tail:       "50",
			}

			out, err := d.cli.ContainerLogs(ctx, containerID, options)
			if err != nil {
				return
			}
			defer out.Close()

			pr, pw := io.Pipe()

			go func() {
				_ = demuxMobyStream(out, pw)
				pw.Close()
			}()

			scanner := bufio.NewScanner(pr)
			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				default:
					line := scanner.Text()
					fmt.Fprintf(writer, "[cyan][%s][-]\t%s\n", serviceName, line)
				}
			}
		}(t.id, t.name)
	}

	<-ctx.Done()
	return nil
}

func (d *Client) GetServicesStatus(ctx context.Context) (map[string]string, error) {
	statusMap := make(map[string]string)

	res, err := d.cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return statusMap, err
	}

	for _, c := range res.Items {
		proj := c.Labels["com.docker.compose.project"]
		svc := c.Labels["com.docker.compose.service"]

		if proj == d.projectName && svc != "" {
			statusMap[svc] = string(c.State)
		}
	}

	return statusMap, nil
}

func (d *Client) IsComposeUp(ctx context.Context) (bool, error) {
	statuses, err := d.GetServicesStatus(ctx)
	if err != nil {
		return false, err
	}
	return len(statuses) > 0, nil
}

func (d *Client) ComposeUp(ctx context.Context, writer io.Writer) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", d.composePath, "-p", d.projectName, "up", "-d")
	cmd.Stdout = writer
	cmd.Stderr = writer
	return cmd.Run()
}

func (d *Client) ComposeDown(ctx context.Context, writer io.Writer) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", d.composePath, "-p", d.projectName, "down")
	cmd.Stdout = writer
	cmd.Stderr = writer
	return cmd.Run()
}

func (d *Client) StartAllContainers(ctx context.Context) error {
	res, err := d.cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return err
	}

	for _, c := range res.Items {
		if c.Labels["com.docker.compose.project"] == d.projectName {
			if string(c.State) != "running" {
				if _, err := d.cli.ContainerStart(ctx, c.ID, client.ContainerStartOptions{}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (d *Client) StopAllContainers(ctx context.Context) error {
	res, err := d.cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return err
	}

	timeout := 10
	for _, c := range res.Items {
		if c.Labels["com.docker.compose.project"] == d.projectName {
			if string(c.State) == "running" {
				if _, err := d.cli.ContainerStop(ctx, c.ID, client.ContainerStopOptions{Timeout: &timeout}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (d *Client) StartContainer(ctx context.Context, serviceName string) error {
	containerID, err := d.findContainerID(ctx, serviceName)
	if err != nil {
		return err
	}
	_, err = d.cli.ContainerStart(ctx, containerID, client.ContainerStartOptions{})
	return err
}

func (d *Client) StopContainer(ctx context.Context, serviceName string) error {
	containerID, err := d.findContainerID(ctx, serviceName)
	if err != nil {
		return err
	}
	timeout := 10
	_, err = d.cli.ContainerStop(ctx, containerID, client.ContainerStopOptions{Timeout: &timeout})
	return err
}

func (d *Client) RestartContainer(ctx context.Context, serviceName string) error {
	containerID, err := d.findContainerID(ctx, serviceName)
	if err != nil {
		return err
	}
	timeout := 10
	_, err = d.cli.ContainerRestart(ctx, containerID, client.ContainerRestartOptions{Timeout: &timeout})
	return err
}

func (d *Client) findContainerID(ctx context.Context, serviceName string) (string, error) {
	res, err := d.cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return "", err
	}

	for _, c := range res.Items {
		proj := c.Labels["com.docker.compose.project"]
		svc := c.Labels["com.docker.compose.service"]

		if proj == d.projectName && svc == serviceName {
			return c.ID, nil
		}
	}
	return "", fmt.Errorf("Container %s not found in %s project", serviceName, d.projectName)
}

func (d *Client) ComposePath() string {
	return d.composePath
}
