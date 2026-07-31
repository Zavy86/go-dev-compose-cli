package config

import (
	"context"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
)

type ServiceInfo struct {
	Name      string
	DependsOn []string
}

func LoadProject(filePath string) (services []ServiceInfo, projectName string, err error) {
	ctx := context.Background()

	var project *types.Project
	project, err = loader.LoadWithContext(ctx, types.ConfigDetails{
		WorkingDir: "",
		ConfigFiles: []types.ConfigFile{
			{Filename: filePath},
		},
	})

	if err != nil {
		return nil, "", err
	}

	projectName = project.Name

	for _, service := range project.Services {
		var deps []string
		for depName := range service.DependsOn {
			deps = append(deps, depName)
		}

		services = append(services, ServiceInfo{
			Name:      service.Name,
			DependsOn: deps,
		})
	}

	return // naked return using named return values
}
