package service

import (
	"github.com/bububa/osssync/internal/config"
)

func Init(cfg *config.Config) {
	SetConfig(cfg)
}

func Start() {
	Syncer().Start(Config())
}

func Close() {
	Syncer().Close()
}

func Reload(cfg *config.Config) {
	Syncer().Reload(cfg)
}
