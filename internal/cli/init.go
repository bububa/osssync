package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service/setup"
)

func init() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s-%s@%s\n", config.AppName, config.GitRevision, config.GitTag)
	}
}

func NewApp(app *cli.App) {
	*app = cli.App{
		Name:    config.AppName,
		Version: config.GitTag,
		Usage:   "sync files to oss",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Load configuration from `FILE`",
			},
		},
		Before: beforeAction,
		After:  afterAction,
		Commands: []*cli.Command{
			{
				Name:     "sync",
				Usage:    "Start syncing",
				Category: "Sync",
				Action:   Sync,
			},
			{
				Name:     "setup",
				Usage:    "setup",
				Category: "Setup",
				Action:   setup.Setup,
			},
		},
	}
}
