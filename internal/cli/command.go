package cli

import (
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"

	"github.com/bububa/osssync/internal/service"
)

func Sync(c *cli.Context) error {
	ctx := c.Context
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	service.Start(ctx)
	<-ctx.Done()
	stop()
	return nil
}
