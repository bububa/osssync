package app

import (
	"os"

	"github.com/urfave/cli/v2"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service"
)

func beforeAction(c *cli.Context) error {
	var cfg config.Config
	if err := loadConfig(c, &cfg); err != nil {
		return err
	}
	/*
		if c.IsSet("pyroscope") {
			profiler.Start(profiler.Config{
				ApplicationName: serverName,
				ServerAddress:   c.String("pyroscope"),
			})
		}
	*/
	service.Init(&cfg)
	// usecase.Setup()
	os.Args = append(os.Args, "--exclude=.*")
	return nil
}

func afterAction(c *cli.Context) error {
	return nil
}
