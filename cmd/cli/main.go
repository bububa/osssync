package main

import (
	"context"
	"log"
	"os"
	"runtime"

	"github.com/urfave/cli/v2"

	app "github.com/bububa/osssync/internal/cli"
	"github.com/bububa/osssync/internal/service"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	ctx := context.Background()
	entry := new(cli.App)
	app.NewApp(entry)
	if err := entry.RunContext(ctx, os.Args); err != nil {
		log.Fatalln(err)
	}
	service.Close()
}
