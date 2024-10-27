package app

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/aliyun/ossutil/lib"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service"
)

func setSystemBar(a fyne.App) {
	desk, _ := a.(desktop.App)
	desk.SetSystemTrayIcon(IconSyncComplete)
	refreshShortCut := &desktop.CustomShortcut{KeyName: fyne.KeyR, Modifier: fyne.KeyModifierControl}
	refreshItem := fyne.NewMenuItem("ForceSync", service.Syncer().Force)
	refreshItem.Shortcut = refreshShortCut
	configShortCut := &desktop.CustomShortcut{KeyName: fyne.KeyC, Modifier: fyne.KeyModifierControl}
	configItem := fyne.NewMenuItem("Configuration", func() { Configure(a) })
	configItem.Shortcut = configShortCut
	menu := fyne.NewMenu(config.AppName,
		refreshItem,
		configItem,
		fyne.NewMenuItem("Log", nil),
	)
	desk.SetSystemTrayMenu(menu)
	go func() {
		for msg := range service.Syncer().Message() {
			var imgName fyne.ThemedResource
			if msg.Status() == lib.MonitorStatusFinish {
				imgName = IconSyncComplete
			} else {
				imgName = IconSyncing
			}
			// menu.Items[0].Label = CPMonitorSnapDesc(msg)
			desk.SetSystemTrayIcon(imgName)
		}
	}()
}
