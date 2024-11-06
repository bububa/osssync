package mount

import (
	"context"
	"fmt"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"github.com/bububa/osssync/pkg/fs/mount/storage"
	"github.com/bububa/osssync/pkg/fs/oss"
)

type Mounter struct {
	ossFS      *oss.FS
	db         storage.Storage
	stopCh     chan struct{}
	exitCh     chan struct{}
	mountpoint string
	name       string
}

func NewMounter(ossFS *oss.FS, mountpoint string, name string, db storage.Storage) *Mounter {
	return &Mounter{
		ossFS:      ossFS,
		db:         db,
		mountpoint: mountpoint,
		name:       name,
		stopCh:     make(chan struct{}, 1),
		exitCh:     make(chan struct{}, 1),
	}
}

func (m *Mounter) Mount(ctx context.Context) error {
	root, err := NewFS(ctx, m.ossFS, m.mountpoint, m.db)
	if err != nil {
		return err
	}
	srv, err := fs.Mount(m.mountpoint, root, &fs.Options{
		MountOptions: fuse.MountOptions{DirectMount: true, SyncRead: true, FsName: fmt.Sprintf("ossfs/%s", m.name), Debug: true},
	})
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		srv.Unmount()
		srv.Wait()
		m.db.Close()
		m.exitCh <- struct{}{}
	}()
	return nil
}

func (m *Mounter) Unmount() {
	m.stopCh <- struct{}{}
	<-m.exitCh
}
