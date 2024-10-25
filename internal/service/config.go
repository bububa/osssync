package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jinzhu/configor"
	gap "github.com/muesli/go-app-paths"

	"github.com/bububa/osssync/internal/config"
)

var configSetting *config.Config

func SetConfig(cfg *config.Config) {
	configSetting = cfg
}

func Config() *config.Config {
	return configSetting
}

func ConfigLoader(cfg *config.Config, configPath string) error {
	loader := configor.New(&configor.Config{
		Environment:          "production",
		ErrorOnUnmatchedKeys: true,
		AutoReload:           true,
		AutoReloadCallback: func(cfg interface{}) {
			Reload(cfg.(*config.Config))
		},
	})
	return loader.Load(cfg, configPath)
}

func LoadConfig(cfg *config.Config) error {
	scope := gap.NewScope(gap.User, config.AppIdentity)
	configPath, err := scope.LookupConfig(config.AppConfig)
	if err != nil {
		return err
	}
	if len(configPath) > 0 {
		return ConfigLoader(cfg, configPath[0])
	}
	return errors.New("no config file found")
}

func LoadConfigString() ([]byte, error) {
	scope := gap.NewScope(gap.User, config.AppIdentity)
	configPath, err := scope.LookupConfig(config.AppConfig)
	if err != nil {
		return nil, err
	}
	if len(configPath) > 0 {
		return os.ReadFile(configPath[0])
	}
	return nil, errors.New("no config file found")
}

func WriteConfigFile(bs []byte) error {
	scope := gap.NewScope(gap.User, config.AppIdentity)
	dirs, err := scope.LookupConfig(config.AppConfig)
	if err != nil {
		return err
	}
	var configPath string
	if len(dirs) == 0 {
		dirs, err = scope.ConfigDirs()
		if err != nil {
			return err
		}
		if err := os.Mkdir(dirs[0], os.ModePerm); err != nil {
			return err
		}
		configPath = filepath.Join(dirs[0], config.AppConfig)
	} else {
		configPath = dirs[0]
	}
	w, err := os.Create(configPath)
	if err != nil {
		fmt.Println(err)
		return err
	}
	if _, err := w.Write(bs); err != nil {
		return err
	}
	return nil
}
