package app

import (
	"fmt"
	"log"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/aliyun/ossutil/lib"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service"
)

func Launch() {
	a := app.New()
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
	setSystemBar(a)
	a.Run()
}

func getPercent(p *lib.CPMonitorSnap) float64 {
	if p.TotalSize() != 0 {
		return float64((p.DealSize())*100.0) / float64(p.TotalSize())
	}
	if p.TotalNum() != 0 {
		return float64((p.DealNum())*100.0) / float64(p.TotalNum())
	}
	return 0
}

func getSpeed(p *lib.CPMonitorSnap) float64 {
	return (float64(p.IncrementSize()) / 1024) / (float64(p.Duration()) * 1e-9)
}

func getSizeString(size int64) string {
	var suffix string
	if size > 1024 || size < -1024 {
		size = size / 1024
		suffix = "KB"
	} else {
		suffix = "B"
	}
	return fmt.Sprintf("%d%s", size, suffix)
}

func CPMonitorSnapDesc(p *lib.CPMonitorSnap) string {
	if p.Status() == lib.MonitorStatusFinish {
		return fmt.Sprintf("%dF,%s", p.OkNum()-p.SkipNum(), getSizeString(p.TransferSize()))
	}
	return fmt.Sprintf("%.3f%%,%.2fKB/s", getPercent(p), getSpeed(p))
}
