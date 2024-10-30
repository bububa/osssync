package sync

import (
	"os"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service/log"
	"github.com/bububa/osssync/pkg/fs/oss"
	"github.com/bububa/osssync/pkg/watcher"
)

type Handler struct {
	fs           *oss.FS
	buffer       map[string]*watcher.Event
	eventCh      chan *watcher.Event
	statusCh     chan<- SyncEvent
	stopCh       chan struct{}
	exitCh       chan struct{}
	closed       bool
	key          string
	enableDelete bool
}

func NewHandler(cfg *config.Setting, statusCh chan<- SyncEvent) (*Handler, error) {
	clt, err := oss.NewClient(cfg.Bucket, cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return nil, err
	}
	fs := oss.NewFS(clt, oss.WithPrefix(cfg.Prefix), oss.WithLocal(cfg.Local))
	h := &Handler{
		key:          cfg.BucketKey(),
		fs:           fs,
		buffer:       make(map[string]*watcher.Event, 1000),
		enableDelete: cfg.Delete,
		statusCh:     statusCh,
		eventCh:      make(chan *watcher.Event, 10000),
		stopCh:       make(chan struct{}, 1),
		exitCh:       make(chan struct{}, 1),
	}
	h.start()
	return h, nil
}

func (h *Handler) Receive(ev *watcher.Event) {
	h.eventCh <- ev
}

func (h *Handler) Key() string {
	return h.key
}

func (h *Handler) start() {
	logger := log.Logger()
	go func() {
		for ev := range h.fs.Events() {
			logger.Warn().Msg(ev.String())
		}
	}()
	ticker := time.NewTicker(500 * time.Millisecond)
	go func() {
		for {
			select {
			case event := <-h.eventCh:
				if event != nil && event.File != nil && !h.closed {
					h.buffer[event.File.Path()] = event
				}
			case <-ticker.C:
				h.process()
			case <-h.stopCh:
				h.closed = true
				ticker.Stop()
				h.fs.Close()
				close(h.exitCh)
				return
			}
		}
	}()
}

func (h *Handler) Close() {
	close(h.stopCh)
	<-h.exitCh
}

func (h *Handler) process() error {
	events := make([]*watcher.Event, 0, len(h.buffer))
	for key, ev := range h.buffer {
		events = append(events, ev)
		delete(h.buffer, key)
	}
	if len(events) > 0 && h.statusCh != nil {
		h.statusCh <- SyncEvent{Handler: h, Status: SyncStart}
		defer func() {
			h.statusCh <- SyncEvent{Handler: h, Status: SyncComplete}
		}()
	}
	return h.handle(events...)
}

func (h *Handler) handle(evs ...*watcher.Event) error {
	deletes := make([]string, 0, len(evs))
	logger := log.Logger()
	for _, ev := range evs {
		l := logger.Warn().Str("file", ev.File.String()).Str("op", ev.Op.String())
		if ev.Ori != nil {
			l.Str("ori", ev.Ori.String())
		}
		l.Msg("fsnotify")
		if ev.Op&fsnotify.Create == fsnotify.Create || ev.Op&fsnotify.Write == fsnotify.Write {
			if _, err := os.Stat(ev.File.Path()); err != nil {
				logger.Error().Err(err).Send()
				continue
			}
			if err := h.fs.Upload(ev.File); err != nil {
				logger.Error().Err(err).Str("op", ev.Op.String()).Str("file", ev.File.Path()).Send()
			}
		} else if ev.Op&fsnotify.Rename == fsnotify.Rename {
			src, err := h.fs.RemotePathFromLocalFile(ev.Ori)
			if err != nil {
				logger.Error().Err(err).Str("op", ev.Op.String()).Str("src", ev.Ori.Path()).Send()
			}
			dist, err := h.fs.RemotePathFromLocalFile(ev.File)
			if err != nil {
				logger.Error().Err(err).Str("op", ev.Op.String()).Str("dist", ev.File.Path()).Send()
			}
			if err := h.fs.Rename(src, dist); err != nil {
				logger.Error().Err(err).Str("op", ev.Op.String()).Str("src", src).Str("dist", dist).Send()
			}
		} else if h.enableDelete && ev.Op&fsnotify.Remove == fsnotify.Remove {
			remotePath, err := h.fs.RemotePathFromLocalFile(ev.File)
			if err != nil {
				logger.Error().Err(err).Str("op", ev.Op.String()).Str("file", ev.File.Path()).Send()
			}
			deletes = append(deletes, remotePath)
		}
	}
	if len(deletes) > 0 {
		h.fs.Remove(deletes...)
	}
	return nil
}
