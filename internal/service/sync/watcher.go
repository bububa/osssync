package sync

import (
	"github.com/fsnotify/fsnotify"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service/log"
	"github.com/bububa/osssync/pkg/watcher"
)

func Watch(cfg *config.Setting, handlers []*Handler) (*watcher.Watcher, error) {
	op := fsnotify.Create | fsnotify.Write | fsnotify.Rename | fsnotify.Remove
	w, err := watcher.NewWatcher(watcher.WithIgnoreHiddenFiles(cfg.IgnoreHiddenFiles), watcher.WithOpFilter(op))
	if err != nil {
		return nil, err
	}
	logger := log.Logger()
	closed := false
	key := cfg.Key()
	go func() {
		for {
			select {
			case event := <-w.Events:
				for _, h := range handlers {
					if closed {
						break
					}
					if event.HandlerKey != "" && event.HandlerKey != h.Key() {
						continue
					}
					event.SettingKey = key
					h.Receive(&event)
				}
			case err := <-w.Errors:
				if err != nil {
					logger.Error().Err(err).Msg("watch")
				}
			case <-w.Closed:
				closed = true
				return
			}
		}
	}()
	if err := w.Start(cfg.Local); err != nil {
		return nil, err
	}
	return w, nil
}
