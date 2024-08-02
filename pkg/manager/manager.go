package manager

import (
	"github.com/moresec-io/conduit/pkg/manager/cms"
	"github.com/moresec-io/conduit/pkg/manager/config"
	"github.com/moresec-io/conduit/pkg/manager/repo"
	"go.uber.org/dig"
)

func BuildContainer() (*dig.Container, error) {
	container := dig.New()
	if err := container.Provide(config.NewConfig); err != nil {
		return nil, err
	}
	if err := container.Provide(repo.NewRepo); err != nil {
		return nil, err
	}
	if err := container.Provide(cms.NewCMS); err != nil {
		return nil, err
	}
	return container, nil
}
