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
	"github.com/bububa/osssync/pkg"
)

func setSystemBar(a fyne.App) {
	desk, _ := a.(desktop.App)
	desk.SetSystemTrayIcon(resource.IconSyncComplete)
	menu := fyne.NewMenu(config.AppName)
	menu.Items = menuItems(a)
	desk.SetSystemTrayMenu(menu)
	syncing := atomic.NewBool(false)
	syncingMap := pkg.NewMap[string, bool]()
	go func() {
		for {
			select {
			case <-service.SystemBarReload():
				menu.Items = menuItems(a)
				menu.Refresh()
			case ev := <-service.Syncer().Events():
				var (
					statusChanged       bool
					updateSyncingStatus bool
					cfgKey              = ev.Handler.ConfigKey()
				)
				if syncing.Load() {
					if ev.Status == sync.SyncComplete {
						syncing.Store(false)
						statusChanged = true
					}
				} else if ev.Status == sync.SyncStart {
					syncing.Store(true)
					statusChanged = true
				}
				if isSyncing, ok := syncingMap.Load(cfgKey); !ok {
					statusChanged = true
					switch ev.Status {
					case sync.SyncStart:
						syncingMap.Store(cfgKey, true)
						updateSyncingStatus = true
					case sync.SyncComplete:
						syncingMap.Store(cfgKey, false)
					}
				} else if ev.Status == sync.SyncStart && !isSyncing {
					syncingMap.Store(cfgKey, true)
					updateSyncingStatus = true
				} else if ev.Status == sync.SyncComplete && isSyncing {
					syncingMap.Store(cfgKey, false)
					updateSyncingStatus = true
				}
				if statusChanged {
					var imgName fyne.ThemedResource
					if syncing.Load() {
						imgName = resource.IconSyncing
					} else {
						imgName = resource.IconSyncComplete
					}
					desk.SetSystemTrayIcon(imgName)
				}
				if updateSyncingStatus {
					for _, m := range menu.Items {
						if subM := m.ChildMenu; subM != nil && subM.Label == cfgKey {
							isSyncing, _ := syncingMap.Load(cfgKey)
							for _, subItem := range subM.Items {
								subItem.Disabled = isSyncing
							}
						}
					}
					menu.Refresh()
				}
			}
		}
	}()
}

func menuItems(a fyne.App) []*fyne.MenuItem {
	addItem := fyne.NewMenuItem(lang.L("systembar.addSetting"), func() { EditSetting(a, config.EmptySetting, true) })
	items := make([]*fyne.MenuItem, 0, len(service.Config().Settings)+4)
	items = append(items, addItem)
	mp := make(map[string]*fyne.MenuItem, len(service.Config().Settings))
	for _, cfg := range service.Config().Settings {
		item := fyne.NewMenuItem(cfg.DisplayName(), nil)
		syncItem := fyne.NewMenuItem(lang.L("systembar.sync"), func() { service.Syncer().Sync(&cfg) })
		syncItem.Icon = theme.UploadIcon()
		editItem := fyne.NewMenuItem(lang.L("systembar.edit"), func() { EditSetting(a, cfg, false) })
		editItem.Icon = theme.SettingsIcon()
		copyItem := fyne.NewMenuItem(lang.L("systembar.duplicate"), func() {
			cfg.Name = ""
			EditSetting(a, cfg, true)
		})
		copyItem.Icon = theme.ContentCopyIcon()
		deleteItem := fyne.NewMenuItem(lang.L("systembar.delete"), func() { deleteConfig(cfg) })
		deleteItem.Icon = theme.DeleteIcon()
		openItem := fyne.NewMenuItem(lang.L("systembar.mount"), func() { mount(&cfg) })
		openItem.Icon = theme.FolderIcon()
		item.ChildMenu = fyne.NewMenu(cfg.Key(),
			syncItem,
			editItem,
			copyItem,
			deleteItem,
			openItem,
		)
		mp[cfg.BucketKey()] = item
		items = append(items, item)
	}
	logItem := fyne.NewMenuItem(lang.L("systembar.log"), func() { LogWindow(a) })
	quitItem := fyne.NewMenuItem(lang.L("systembar.quit"), nil)
	quitItem.IsQuit = true
	items = append(items, logItem, fyne.NewMenuItemSeparator(), quitItem)
	return items
}

func mount(cfg *config.Setting) {
	service.Syncer().Mount(cfg)
}
