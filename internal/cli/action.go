package cli

import (
	"github.com/urfave/cli/v2"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service"
)

func beforeAction(c *cli.Context) error {
	var cfg config.Config
	if err := loadConfig(c, &cfg); err != nil {
		return err
	}
	service.Init(&cfg)
	return nil
}

func afterAction(c *cli.Context) error {
	return nil
}
