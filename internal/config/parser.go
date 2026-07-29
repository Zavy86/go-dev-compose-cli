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

func LoadProject(filePath string) ([]ServiceInfo, string, error) {
	ctx := context.Background()

	project, err := loader.LoadWithContext(ctx, types.ConfigDetails{
		WorkingDir: "",
		ConfigFiles: []types.ConfigFile{
			{Filename: filePath},
		},
	})
	if err != nil {
		return nil, "", err
	}

	var services []ServiceInfo

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

	return services, project.Name, nil
}
