package service

import "github.com/bububa/osssync/internal/config"

var Config *config.Config

func Init(cfg *config.Config) {
	Config = cfg
}
