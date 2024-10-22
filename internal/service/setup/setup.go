package setup

import (
	"os"

	"github.com/urfave/cli/v2"

	"github.com/bububa/osssync/internal/config"
)

func Setup(c *cli.Context) error {
	configPath := c.String("config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return err
	}
	bs, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	return config.WriteConfigFile(bs)
}
