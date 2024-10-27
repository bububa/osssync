package setup

import (
	"os"

	"github.com/bububa/osssync/internal/service"
	"github.com/urfave/cli/v2"
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
	_, err = service.WriteConfigFile(bs)
	return err
}
