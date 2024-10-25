package cli

import (
	"github.com/urfave/cli/v2"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service"
)

func loadConfig(c *cli.Context, cfg *config.Config) error {
	configPath := c.String("config")
	if configPath != "" {
		service.ConfigLoader(cfg, configPath)
	}
	return service.LoadConfig(cfg)
}
