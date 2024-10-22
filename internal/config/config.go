package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jinzhu/configor"
	gap "github.com/muesli/go-app-paths"
)

func LoadConfig(cfg *Config) error {
	scope := gap.NewScope(gap.User, AppIdentity)
	configPath, err := scope.LookupConfig(AppConfig)
	if err != nil {
		return err
	}
	if len(configPath) > 0 {
		loader := configor.New(&configor.Config{
			Environment:          "production",
			ErrorOnUnmatchedKeys: true,
		})
		return loader.Load(cfg, configPath[0])
	}
	return errors.New("no config file found")
}

func WriteConfigFile(bs []byte) error {
	scope := gap.NewScope(gap.User, AppIdentity)
	dirs, err := scope.LookupConfig(AppConfig)
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
		configPath = filepath.Join(dirs[0], AppConfig)
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

type Config struct {
	Settings []Setting `required:"true"`
}

type Setting struct {
	Local string `required:"true"`
	Credential
	IgnoreHiddenFiles bool
	Delete            bool
}

type Credential struct {
	Endpoint        string `required:"true"`
	AccessKeyID     string `required:"true"`
	AccessKeySecret string `required:"true"`
	Bucket          string `required:"true"`
	Prefix          string
}
