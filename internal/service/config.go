package service

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/jinzhu/configor"

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
	configPath, err := xdg.ConfigFile(filepath.Join(config.AppIdentity, config.AppConfig))
	if err != nil {
		return err
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := WriteConfigFile(configPath, nil); err != nil {
			return err
		}
	}
	return ConfigLoader(cfg, configPath)
}

func SaveConfig(cfg *config.Config) error {
	configPath, err := xdg.ConfigFile(filepath.Join(config.AppIdentity, config.AppConfig))
	if err != nil {
		return err
	}
	w, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer w.Close()
	return template.Template().ExecuteTemplate(w, "config.tpl", cfg)
}

func WriteConfigFile(configPath string, bs []byte) error {
	w, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer w.Close()
	if _, err := w.Write(bs); err != nil {
		return err
	}
	return nil
}
