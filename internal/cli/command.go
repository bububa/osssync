package cli

import (
	"log"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"

	"github.com/bububa/osssync/internal/service"
)

func Sync(c *cli.Context) error {
	ctx := c.Context
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	service.Start()
	go func() {
		for msg := range service.Syncer().Message() {
			log.Println("[syncer message] status:", msg.Status(), ", files: ", msg.FileNum())
		}
	}()
	<-ctx.Done()
	stop()
	return nil
}
