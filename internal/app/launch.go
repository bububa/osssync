package app

import (
	"embed"
	"log"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/lang"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service"
)

//go:embed lang
var i18n embed.FS

func Launch() {
	a := app.NewWithID(config.AppIdentity)
	if _, ok := a.(desktop.App); !ok {
		log.Fatalln("invalid platform")
	}
	cfg := new(config.Config)
	if err := service.LoadConfig(cfg); err != nil {
		log.Fatalln(err)
	}
	service.Init(cfg)
	service.Start()
	defer service.Close()
	lang.AddTranslationsFS(i18n, "lang")
	setSystemBar(a)
	a.Run()
}
