package app

import (
	"github.com/jinzhu/configor"
	"github.com/urfave/cli/v2"

	"github.com/bububa/osssync/internal/config"
)

func loadConfig(c *cli.Context, cfg *config.Config) error {
	configPath := c.String("config")
	if configPath != "" {
		loader := configor.New(&configor.Config{
			Environment:          "production",
			ErrorOnUnmatchedKeys: true,
		})
		return loader.Load(cfg, configPath)
	}
	return config.LoadConfig(cfg)
}
