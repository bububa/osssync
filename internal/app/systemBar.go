package app

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"go.uber.org/atomic"

	"github.com/bububa/osssync/internal/app/resource"
	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service"
	"github.com/bububa/osssync/internal/service/sync"
)

func setSystemBar(a fyne.App) {
	desk, _ := a.(desktop.App)
	desk.SetSystemTrayIcon(resource.IconSyncComplete)
	menu := fyne.NewMenu(config.AppName)
	menu.Items = menuItems(a)
	desk.SetSystemTrayMenu(menu)
	syncing := atomic.NewBool(false)
	go func() {
		for {
			select {
			case <-service.SystemBarReload():
				menu.Items = menuItems(a)
				menu.Refresh()
			case ev := <-service.Syncer().Events():
				var (
					statusChanged bool
					status        = syncing.Load()
				)
				if ev.Status == sync.SyncStart && !status {
					syncing.Store(true)
					statusChanged = true
				} else if ev.Status == sync.SyncComplete && status {
					syncing.Store(false)
					statusChanged = true
				}
				if statusChanged {
					var imgName fyne.ThemedResource
					if status {
						imgName = resource.IconSyncing
					} else {
						imgName = resource.IconSyncComplete
					}
					desk.SetSystemTrayIcon(imgName)
				}
			}
		}
	}()
}

func menuItems(a fyne.App) []*fyne.MenuItem {
	addItem := fyne.NewMenuItem(lang.L("systembar.addSetting"), func() { EditSetting(a, config.Setting{}, createConfig) })
	items := make([]*fyne.MenuItem, 0, len(service.Config().Settings)+4)
	items = append(items, addItem)
	for _, cfg := range service.Config().Settings {
		item := fyne.NewMenuItem(cfg.DisplayName(), nil)
		syncItem := fyne.NewMenuItem(lang.L("systembar.sync"), func() { service.Syncer().Sync(&cfg) })
		syncItem.Icon = theme.UploadIcon()
		editItem := fyne.NewMenuItem(lang.L("systembar.edit"), func() { EditSetting(a, cfg, updateConfig) })
		editItem.Icon = theme.SettingsIcon()
		copyItem := fyne.NewMenuItem(lang.L("systembar.duplicate"), func() {
			cfg.Name = ""
			EditSetting(a, cfg, createConfig)
		})
		copyItem.Icon = theme.ContentCopyIcon()
		deleteItem := fyne.NewMenuItem(lang.L("systembar.delete"), func() { deleteConfig(cfg) })
		deleteItem.Icon = theme.DeleteIcon()
		item.ChildMenu = fyne.NewMenu(cfg.Key(),
			syncItem,
			editItem,
			copyItem,
			deleteItem,
		)
		items = append(items, item)
	}
	logItem := fyne.NewMenuItem(lang.L("systembar.log"), func() { LogWindow(a) })
	quitItem := fyne.NewMenuItem(lang.L("systembar.quit"), nil)
	quitItem.IsQuit = true
	items = append(items, logItem, fyne.NewMenuItemSeparator(), quitItem)
	return items
}
