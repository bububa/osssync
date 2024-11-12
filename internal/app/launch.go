package app

import (
	"context"
	"embed"
	"log"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/lang"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service"
	"github.com/bububa/osssync/pkg"
)

//go:embed lang
var i18n embed.FS

func Launch(ctx context.Context) {
	a := app.NewWithID(pkg.AppIdentity)
	if _, ok := a.(desktop.App); !ok {
		log.Fatalln("invalid platform")
	}
	cfg := new(config.Config)
	if err := service.LoadConfig(cfg); err != nil {
		log.Fatalln(err)
	}
	service.Init(cfg)
	service.Start(ctx)
	defer service.Close()
	lang.AddTranslationsFS(i18n, "lang")
	setSystemBar(a)
	a.Run()
}
