package sync

import (
	"go.uber.org/atomic"

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
	eventCh  chan SyncEvent
	stopCh   chan struct{}
	exitCh   chan struct{}
	watchers map[string]*watcher.Watcher
	closed   *atomic.Bool
	handlers []*Handler
}

func NewSyncer() *Syncer {
	return &Syncer{
		eventCh:  make(chan SyncEvent, 1000),
		closed:   atomic.NewBool(false),
		reloadCh: make(chan *config.Config, 1),
		stopCh:   make(chan struct{}, 1),
		exitCh:   make(chan struct{}, 1),
	}
}

func (s *Syncer) Start(cfg *config.Config) error {
	if err := s.start(cfg); err != nil {
		return err
	}
	logger := log.Logger()
	go func() {
		for {
			select {
			case cfg := <-s.reloadCh:
				if s.closed.Load() {
					return
				}
				if err := s.reload(cfg); err != nil {
					logger.Error().Err(err).Msg("reload")
				}
			case <-s.stopCh:
				s.closed.Store(true)
				s.stop()
				close(s.reloadCh)
				close(s.eventCh)
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
	handlerKey := cfg.BucketKey()
	if w, ok := s.watchers[cfg.Local]; ok {
		w.Notify(handlerKey)
	}
}

func (s *Syncer) Events() <-chan SyncEvent {
	return s.eventCh
}

func (s *Syncer) start(cfg *config.Config) error {
	settings := cfg.Settings
	watchers := make(map[string]*watcher.Watcher, len(settings))
	handlers := make(map[string][]*Handler, len(settings))
	s.handlers = make([]*Handler, 0, len(settings))
	for _, setting := range settings {
		key := setting.Local
		if h, err := NewHandler(&setting, s.eventCh); err != nil {
			return err
		} else {
			handlers[key] = append(handlers[key], h)
			s.handlers = append(s.handlers, h)
		}
	}
	for _, setting := range settings {
		key := setting.Local
		if _, ok := watchers[key]; !ok {
			if w, err := Watch(&setting, handlers[key]); err != nil {
				return err
			} else {
				watchers[key] = w
			}
		}
	}
	s.watchers = watchers
	return nil
}

func (s *Syncer) reload(cfg *config.Config) error {
	s.stop()
	return s.start(cfg)
}

func (s *Syncer) stop() {
	for _, w := range s.watchers {
		w.Close()
	}
	for _, h := range s.handlers {
		h.Close()
	}
	s.handlers = nil
	s.watchers = nil
}

func (s *Syncer) Close() {
	close(s.stopCh)
	<-s.exitCh
}
