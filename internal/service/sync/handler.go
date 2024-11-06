package sync

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/atomic"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service/log"
	"github.com/bububa/osssync/pkg"
	"github.com/bububa/osssync/pkg/fs/oss"
	"github.com/bububa/osssync/pkg/watcher"
)

type Handler struct {
	fs           *oss.FS
	buffer       *pkg.Map[string, *watcher.Event]
	eventCh      chan *watcher.Event
	statusCh     chan<- SyncEvent
	stopCh       chan struct{}
	exitCh       chan struct{}
	closed       *atomic.Bool
	cfg          *config.Setting
	enableDelete bool
}

func NewHandler(cfg *config.Setting, statusCh chan<- SyncEvent) (*Handler, error) {
	clt, err := oss.NewClient(cfg.Bucket, cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return nil, err
	}
	fs := oss.NewFS(clt, oss.WithPrefix(cfg.Prefix), oss.WithLocal(cfg.Local))
	h := &Handler{
		cfg:          cfg,
		fs:           fs,
		buffer:       pkg.NewMap[string, *watcher.Event](),
		enableDelete: cfg.Delete,
		statusCh:     statusCh,
		eventCh:      make(chan *watcher.Event, 10000),
		stopCh:       make(chan struct{}, 1),
		exitCh:       make(chan struct{}, 1),
		closed:       atomic.NewBool(false),
	}
	h.start()
	return h, nil
}

func (h *Handler) Receive(ev *watcher.Event) {
	if h.closed.Load() {
		return
	}
	h.eventCh <- ev
}

func (h *Handler) Key() string {
	return h.cfg.BucketKey()
}

func (h *Handler) ConfigKey() string {
	return h.cfg.Key()
}

func (h *Handler) FS() *oss.FS {
	return h.fs
}

func (h *Handler) HasChange(cfg *config.Setting) bool {
	return h.cfg.Credential != cfg.Credential || h.cfg.IgnoreHiddenFiles != cfg.IgnoreHiddenFiles || h.cfg.Delete != cfg.Delete
}

func (h *Handler) start() {
	logger := log.Logger()
	go func() {
		for ev := range h.fs.Events() {
			logger.Warn().Msg(ev.String())
		}
	}()
	ticker := time.NewTicker(500 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	mounter, err := Mount(ctx, h.cfg)
	if err != nil {
		fmt.Println(err)
	} else {
		defer mounter.Unmount()
	}
	go func() {
		for {
			select {
			case <-ticker.C:
				h.process(ctx)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
	go func() {
		for {
			select {
			case event := <-h.eventCh:
				if event != nil && event.File != nil {
					h.buffer.Store(event.File.Path(), event)
				}
			case <-h.stopCh:
				cancel()
				h.closed.Store(true)
				close(h.eventCh)
				h.fs.Close()
				close(h.exitCh)
				return
			}
		}
	}()
}

func (h *Handler) Close() {
	fmt.Println("handler closed")
	close(h.stopCh)
	<-h.exitCh
}

func (h *Handler) process(ctx context.Context) error {
	var events []*watcher.Event
	h.buffer.Range(func(key string, ev *watcher.Event) bool {
		events = append(events, ev)
		h.buffer.Delete(key)
		return true
	})
	if len(events) > 0 && h.statusCh != nil {
		h.statusCh <- SyncEvent{Handler: h, Status: SyncStart}
		defer func() {
			h.statusCh <- SyncEvent{Handler: h, Status: SyncComplete}
		}()
	}
	return h.handle(ctx, events...)
}

func (h *Handler) handle(ctx context.Context, evs ...*watcher.Event) error {
	deletes := make([]string, 0, len(evs))
	logger := log.Logger()
	pool := pond.NewPool(10)
	group := pool.NewGroup()
	for _, ev := range evs {
		l := logger.Warn().Str("file", ev.File.String()).Str("op", ev.Op.String())
		if ev.Ori != nil {
			l.Str("ori", ev.Ori.String())
		}
		l.Msg("fsnotify")
		if h.enableDelete && ev.Op&fsnotify.Remove == fsnotify.Remove {
			remotePath, err := h.fs.RemotePathFromLocalFile(ev.File)
			if err != nil {
				logger.Error().Err(err).Str("op", ev.Op.String()).Str("file", ev.File.Path()).Send()
			}
			deletes = append(deletes, remotePath)
		} else {
			ev := ev
			group.Submit(func() {
				h.eventHandler(ctx, ev)
			})
		}
	}
	if len(deletes) > 0 {
		group.Submit(func() {
			h.fs.Remove(ctx, deletes...)
		})
	}
	return group.Wait()
}

func (h *Handler) eventHandler(ctx context.Context, ev *watcher.Event) error {
	logger := log.Logger()
	if ev.Op&fsnotify.Create == fsnotify.Create || ev.Op&fsnotify.Write == fsnotify.Write {
		if _, err := os.Stat(ev.File.Path()); err != nil {
			logger.Error().Err(err).Send()
			return err
		}
		if err := h.fs.UploadFile(ctx, ev.File); err != nil {
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
		if err := h.fs.Rename(ctx, src, dist); err != nil {
			logger.Error().Err(err).Str("op", ev.Op.String()).Str("src", src).Str("dist", dist).Send()
		}
	}
	return nil
}
