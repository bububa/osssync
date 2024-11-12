package sync

import (
	"context"
	"errors"
	"fmt"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service/log"
	"github.com/bububa/osssync/pkg/watcher"
)

type SyncStatus int32

const (
	SyncComplete SyncStatus = iota
	SyncStart
)

type SyncEvent struct {
	Handler    *Handler
	SettingKey string
	Status     SyncStatus
}

type Syncer struct {
	reloadCh chan *config.Config
	syncCh   chan *config.Setting
	mountCh  chan *config.Setting
	eventCh  chan SyncEvent
	stopCh   chan struct{}
	exitCh   chan struct{}
	watchers map[string]*watcher.Watcher
	handlers map[string]*Handler
	closed   bool
}

func NewSyncer() *Syncer {
	return &Syncer{
		watchers: make(map[string]*watcher.Watcher),
		handlers: make(map[string]*Handler),
		eventCh:  make(chan SyncEvent, 1000),
		syncCh:   make(chan *config.Setting, 1),
		mountCh:  make(chan *config.Setting, 1),
		reloadCh: make(chan *config.Config, 1),
		stopCh:   make(chan struct{}, 1),
		exitCh:   make(chan struct{}, 1),
	}
}

func (s *Syncer) Start(ctx context.Context, cfg *config.Config) error {
	if err := s.start(ctx, cfg); err != nil {
		return err
	}
	logger := log.Logger()
	go func() {
		for {
			select {
			case cfg := <-s.reloadCh:
				if s.closed || cfg == nil {
					return
				}
				if err := s.reload(ctx, cfg); err != nil {
					logger.Error().Err(err).Msg("reload")
				}
			case setting := <-s.syncCh:
				if s.closed || setting == nil {
					return
				}
				s.sync(setting)
			case setting := <-s.mountCh:
				if s.closed || setting == nil {
					return
				}
				s.mount(setting)
			case <-s.stopCh:
				s.closed = true
				s.stop(nil)
				close(s.reloadCh)
				close(s.eventCh)
				close(s.syncCh)
				close(s.mountCh)
				close(s.exitCh)
				return
			}
		}
	}()
	return nil
}

func (s *Syncer) Reload(cfg *config.Config) {
	s.reloadCh <- cfg
}

func (s *Syncer) SyncAll() {
	for _, w := range s.watchers {
		w.Notify("")
	}
}

func (s *Syncer) Sync(cfg *config.Setting) {
	s.syncCh <- cfg
}

func (s *Syncer) Mount(cfg *config.Setting) {
	s.mountCh <- cfg
}

func (s *Syncer) Events() <-chan SyncEvent {
	return s.eventCh
}

func (s *Syncer) start(ctx context.Context, cfg *config.Config) error {
	settings := cfg.Settings
	handlers := make(map[string][]*Handler, len(settings))
	s.watchers = make(map[string]*watcher.Watcher, len(settings))
	for _, setting := range settings {
		key := setting.Local
		bucketKey := setting.BucketKey()
		if h, ok := s.handlers[bucketKey]; ok && !h.HasChange(&setting) {
			handlers[key] = append(handlers[key], s.handlers[bucketKey])
		} else {
			if h, err := NewHandler(&setting, s.eventCh); err != nil {
				return err
			} else {
				handlers[key] = append(handlers[key], h)
				s.handlers[bucketKey] = h
			}
		}
	}
	for _, setting := range settings {
		key := setting.Local
		if _, ok := s.watchers[key]; !ok {
			if w, err := Watch(&setting, handlers[key]); err != nil {
				return err
			} else {
				s.watchers[key] = w
			}
		}
	}
	return nil
}

func (s *Syncer) reload(ctx context.Context, cfg *config.Config) error {
	s.stop(cfg)
	return s.start(ctx, cfg)
}

func (s *Syncer) stop(cfg *config.Config) {
	for _, w := range s.watchers {
		w.Close()
	}
	s.watchers = nil
	for bucketKey, h := range s.handlers {
		if cfg != nil {
			for _, setting := range cfg.Settings {
				if !h.HasChange(&setting) {
					h.Close()
					delete(s.handlers, bucketKey)
				}
			}
		} else {
			h.Close()
		}
	}
}

func (s *Syncer) sync(cfg *config.Setting) {
	handlerKey := cfg.BucketKey()
	if w, ok := s.watchers[cfg.Local]; ok {
		w.Notify(handlerKey)
	}
}

func (s *Syncer) mount(cfg *config.Setting) error {
	h, ok := s.handlers[cfg.BucketKey()]
	if !ok {
		return errors.New("handler not exists")
	}
	return h.Mount()
}

func (s *Syncer) Close() {
	fmt.Println("syncer close")
	close(s.stopCh)
	<-s.exitCh
}
