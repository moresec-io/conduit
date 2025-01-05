package manager

import (
	"github.com/moresec-io/conduit/pkg/manager/cms"
	"github.com/moresec-io/conduit/pkg/manager/config"
	"github.com/moresec-io/conduit/pkg/manager/repo"
	"github.com/moresec-io/conduit/pkg/manager/service"
	"github.com/singchia/go-timer/v2"
	"go.uber.org/dig"
)

func BuildContainer() (*dig.Container, error) {
	container := dig.New()
	// provide config
	if err := container.Provide(config.NewConfig); err != nil {
		return nil, err
	}
	// provide timer
	if err := container.Provide(timer.NewTimer); err != nil {
		return nil, err
	}
	// provide repo
	if err := container.Provide(repo.NewRepo); err != nil {
		return nil, err
	}
	// provide cert manager service
	if err := container.Provide(cms.NewCMS); err != nil {
		return nil, err
	}
	// provide conduit manager
	if err := container.Provide(service.NewConduitManager); err != nil {
		return nil, err
	}
	return container, nil
}
