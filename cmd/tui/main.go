package main

import (
	"fmt"

	"github.com/Zavy86/go-dev-compose-cli/internal/config"
	"github.com/Zavy86/go-dev-compose-cli/internal/docker"
	"github.com/Zavy86/go-dev-compose-cli/internal/tui"
)

func main() {
	composeFile, err := config.FindComposeFile("")
	var services []config.ServiceInfo
	var projectName string
	dockerStatus := ""

	if err != nil {
		dockerStatus += fmt.Sprintf("[red]Compose Error: %v[-]\n", err)
	} else {
		dockerStatus += fmt.Sprintf("[green]Found: %s[-]\n", composeFile)
		parsedServices, name, err := config.LoadProject(composeFile)
		if err != nil {
			dockerStatus += fmt.Sprintf("[red]Parsing Error: %v[-]\n", err)
		} else {
			services = parsedServices
			projectName = name
		}
	}

	dockerCli, err := docker.NewClient(composeFile, projectName)
	if err != nil {
		dockerStatus += fmt.Sprintf("[red]Docker Client Error: %v[-]\n", err)
	} else {
		dockerStatus += fmt.Sprintf("[green]Docker Client: Connected (Progetto: %s)[-]\n", projectName)
	}

	ui := tui.NewUI(services, dockerStatus, dockerCli)
	if err := ui.App.Run(); err != nil {
		panic(err)
	}
}
