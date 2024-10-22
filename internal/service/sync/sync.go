package sync

import (
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"

	"github.com/bububa/osssync/internal/service"
	"github.com/bububa/osssync/pkg/watcher"
)

func Sync(c *cli.Context) error {
	ctx := c.Context
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	settings := service.Config.Settings
	watchers := make([]*watcher.Watcher, 0, len(settings))
	for _, setting := range settings {
		if w, err := Watch(&setting); err != nil {
			return err
		} else {
			watchers = append(watchers, w)
		}
	}
	<-ctx.Done()
	stop()
	for _, w := range watchers {
		w.Close()
	}
	return nil
}
