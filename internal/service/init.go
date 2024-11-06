package service

import (
	"context"

	"github.com/bububa/osssync/internal/config"
)

var systemBarReloadCh = make(chan struct{}, 1)

func Init(cfg *config.Config) {
	SetConfig(cfg)
}

func Start(ctx context.Context) {
	Syncer().Start(ctx, Config())
}

func Close() {
	Syncer().Close()
	close(systemBarReloadCh)
}

func Reload(cfg *config.Config) {
	systemBarReloadCh <- struct{}{}
	Syncer().Reload(cfg)
}

func SystemBarReload() <-chan struct{} {
	return systemBarReloadCh
}
