package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/bububa/osssync/pkg"
)

func init() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s-%s@%s\n", pkg.AppName, pkg.GitRevision, pkg.GitTag)
	}
}

func NewApp(app *cli.App) {
	*app = cli.App{
		Name:    pkg.AppName,
		Version: pkg.GitTag,
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
		},
	}
}
