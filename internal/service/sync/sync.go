package sync

import (
	"log"

	"go.uber.org/atomic"

	"github.com/aliyun/ossutil/lib"
	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/pkg/watcher"
)

type Syncer struct {
	reloadCh chan *config.Config
	msgCh    chan *lib.CPMonitorSnap
	closed   *atomic.Bool
	watchers []*watcher.Watcher
}

func NewSyncer() *Syncer {
	return &Syncer{
		closed:   atomic.NewBool(false),
		msgCh:    make(chan *lib.CPMonitorSnap, 10000),
		reloadCh: make(chan *config.Config, 1),
	}
}

func (s *Syncer) Start(cfg *config.Config) error {
	if err := s.start(cfg); err != nil {
		return err
	}
	go func() {
		for cfg := range s.reloadCh {
			if s.closed.Load() {
				return
			}
			if err := s.reload(cfg); err != nil {
				log.Println(err)
			}
		}
	}()
	return nil
}

func (s *Syncer) Reload(cfg *config.Config) {
	s.reloadCh <- cfg
}

func (s *Syncer) Message() <-chan *lib.CPMonitorSnap {
	return s.msgCh
}

func (s *Syncer) Force() {
	for _, w := range s.watchers {
		w.Notify()
	}
}

func (s *Syncer) start(cfg *config.Config) error {
	settings := cfg.Settings
	watchers := make([]*watcher.Watcher, 0, len(settings))
	for _, setting := range settings {
		if w, err := Watch(&setting, s.msgCh); err != nil {
			return err
		} else {
			watchers = append(watchers, w)
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
	s.watchers = nil
}

func (s *Syncer) Close() {
	s.closed.Store(true)
	s.stop()
	close(s.reloadCh)
	close(s.msgCh)
}
