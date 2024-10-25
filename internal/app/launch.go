//go:build darwin
// +build darwin

package app

import (
	"fmt"
	"log"
	"strings"

	"github.com/aliyun/ossutil/lib"
	"github.com/progrium/darwinkit/dispatch"
	"github.com/progrium/darwinkit/helper/widgets"
	"github.com/progrium/darwinkit/macos/appkit"
	"github.com/progrium/darwinkit/macos/foundation"
	"github.com/progrium/darwinkit/objc"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service"
)

func Launch(app appkit.Application, delegate *appkit.ApplicationDelegate) {
	app.SetActivationPolicy(appkit.ApplicationActivationPolicyRegular)
	app.ActivateIgnoringOtherApps(true)
	delegate.SetApplicationShouldTerminateAfterLastWindowClosed(func(appkit.Application) bool {
		return false
	})
	cfg := new(config.Config)
	if err := service.LoadConfig(cfg); err != nil {
		log.Fatalln(err)
	}
	service.Init(cfg)
	service.Start()
	setSystemBar(app)
}

func setSystemBar(app appkit.Application) {
	item := appkit.StatusBar_SystemStatusBar().StatusItemWithLength(appkit.VariableStatusItemLength)
	objc.Retain(&item)
	item.Button().SetImagePosition(appkit.ImageLeading)
	if config.Bundled != "" {
		img := appkit.Image_ImageNamed("syncing")
		item.Button().SetImage(img)
	} else {
		img := appkit.Image_ImageWithSystemSymbolNameAccessibilityDescription("icloud", "A multiply symbol inside a filled circle.")
		item.Button().SetImage(img)
	}
	item.Button().SetFont(appkit.Font_MonospacedSystemFontOfSizeWeight(10.0, appkit.FontWeightMedium))

	menu := appkit.NewMenuWithTitle("main")
	menu.AddItem(appkit.NewMenuItemWithAction("Force Sync", "s", func(sender objc.Object) { service.Syncer().Force() }))
	menu.AddItem(appkit.NewMenuItemWithAction("Configuration", "c", func(sender objc.Object) { openConfig() }))
	// menu.AddItem(appkit.NewMenuItemWithAction("Hide", "h", func(sender objc.Object) { app.Hide(nil) }))
	menu.AddItem(appkit.NewMenuItemWithAction("Quit", "q", func(sender objc.Object) { app.Terminate(nil) }))
	item.SetMenu(menu)
	go func() {
		for msg := range service.Syncer().Message() {
			dispatch.MainQueue().DispatchAsync(func() {
				var imgName appkit.ImageName
				if config.Bundled != "" {
					if msg.Status() == lib.MonitorStatusFinish {
						imgName = "syncing_complete"
					} else {
						imgName = "syncing"
					}
				} else {
					if msg.Status() == lib.MonitorStatusFinish {
						imgName = "checkmark.icloud.fill"
					} else {
						imgName = "icloud.fill"
					}
				}
				if config.Bundled != "" {
					img := appkit.Image_ImageNamed(imgName)
					item.Button().SetImage(img)
				} else {

					img := appkit.Image_ImageWithSystemSymbolNameAccessibilityDescription(string(imgName), "A multiply symbol inside a filled circle.")
					item.Button().SetImage(img)
				}
				item.Button().SetTitle(CPMonitorSnapDesc(msg))
			})
		}
	}()
}

func openConfig() {
	configData, _ := service.LoadConfigString()
	w := widgets.NewDialog(400, 300)
	w.SetTitle("OSS Sync Configuration")
	w.SetHidesOnDeactivate(false)
	textView := appkit.TextView_ScrollableTextView()
	textView.SetTranslatesAutoresizingMaskIntoConstraints(false)
	tv := appkit.TextViewFrom(textView.DocumentView().Ptr())
	tv.SetAllowsUndo(true)
	tv.SetRichText(false)
	tv.SetString(string(configData))
	w.SetView(textView)
	w.Show(func() {
		content := tv.String()
		service.WriteConfigFile([]byte(strings.TrimSpace(content)))
	})
	w.MakeKeyAndOrderFront(nil)
	w.Center()
}

func rectOf(x, y, width, height float64) foundation.Rect {
	return foundation.Rect{Origin: foundation.Point{X: x, Y: y}, Size: foundation.Size{Width: width, Height: height}}
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
