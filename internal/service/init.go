package service

import (
	"github.com/bububa/osssync/internal/config"
)

var systemBarReloadCh = make(chan struct{}, 1)

func Init(cfg *config.Config) {
	SetConfig(cfg)
}

func Start() {
	Syncer().Start(Config())
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
