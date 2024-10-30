package service

import (
	"os"
	"path/filepath"

	"github.com/jinzhu/configor"
	gap "github.com/muesli/go-app-paths"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/config/template"
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
	cfgPath, err := WriteConfigFile(nil)
	if err != nil {
		return err
	}
	return ConfigLoader(cfg, cfgPath)
}

func SaveConfig(cfg *config.Config) error {
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
		return err
	}
	defer w.Close()
	return template.Template().ExecuteTemplate(w, "config.tpl", cfg)
}

func WriteConfigFile(bs []byte) (string, error) {
	scope := gap.NewScope(gap.User, config.AppIdentity)
	dirs, err := scope.LookupConfig(config.AppConfig)
	if err != nil {
		return "", err
	}
	var configPath string
	if len(dirs) == 0 {
		dirs, err = scope.ConfigDirs()
		if err != nil {
			return "", err
		}
		if err := os.Mkdir(dirs[0], os.ModePerm); err != nil {
			return "", err
		}
		configPath = filepath.Join(dirs[0], config.AppConfig)
	} else {
		configPath = dirs[0]
	}
	w, err := os.Create(configPath)
	if err != nil {
		return "", err
	}
	defer w.Close()
	if _, err := w.Write(bs); err != nil {
		return "", err
	}
	return configPath, nil
}
