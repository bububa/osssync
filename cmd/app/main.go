package main

import (
	"context"
	"log"
	"os"
	"runtime"

	"github.com/urfave/cli/v2"

	"github.com/bububa/osssync/internal/app"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	ctx := context.Background()
	entry := new(cli.App)
	app.NewApp(entry)
	if err := entry.RunContext(ctx, os.Args); err != nil {
		log.Fatalln(err)
	}
}
