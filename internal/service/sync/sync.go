package sync

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/atomic"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service/log"
	"github.com/bububa/osssync/pkg/fs/oss"
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
	reloadCh    chan *config.Config
	eventCh     chan SyncEvent
	stopCh      chan struct{}
	exitCh      chan struct{}
	watchers    map[string]*watcher.Watcher
	handlers    map[string]*Handler
	closed      *atomic.Bool
	configCache map[string]config.Setting
}

func NewSyncer() *Syncer {
	return &Syncer{
		configCache: make(map[string]config.Setting),
		watchers:    make(map[string]*watcher.Watcher),
		handlers:    make(map[string]*Handler),
		eventCh:     make(chan SyncEvent, 1000),
		closed:      atomic.NewBool(false),
		reloadCh:    make(chan *config.Config, 1),
		stopCh:      make(chan struct{}, 1),
		exitCh:      make(chan struct{}, 1),
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
				if s.closed.Load() {
					return
				}
				if err := s.reload(ctx, cfg); err != nil {
					logger.Error().Err(err).Msg("reload")
				}
			case <-s.stopCh:
				s.closed.Store(true)
				s.stop(nil)
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

func (s *Syncer) FSByConfig(cfg *config.Setting) (*oss.FS, error) {
	h, ok := s.handlers[cfg.BucketKey()]
	if !ok {
		return nil, errors.New("handler not exists")
	}
	return h.fs, nil
}

func (s *Syncer) Close() {
	fmt.Println("syncer close")
	close(s.stopCh)
	<-s.exitCh
}
