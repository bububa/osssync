package mount

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

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
	srv, err := mount(m.mountpoint, root, &fs.Options{
		MountOptions: fuse.MountOptions{DirectMount: true, DirectMountStrict: true, SyncRead: true, FsName: fmt.Sprintf("ossfs/%s", m.name), AllowOther: false, Debug: false, Options: []string{"rw"}},
	})
	if err != nil && srv == nil {
		return err
	}
	go func() {
		<-m.stopCh
		srv.Unmount()
		srv.Wait()
		m.db.Close()
		m.exitCh <- struct{}{}
	}()
	return nil
}

func mount(dir string, root fs.InodeEmbedder, options *fs.Options) (*fuse.Server, error) {
	if options == nil {
		one := time.Second
		options = &fs.Options{
			EntryTimeout: &one,
			AttrTimeout:  &one,
		}
	}

	rawFS := fs.NewNodeFS(root, options)
	server, err := fuse.NewServer(rawFS, dir, &options.MountOptions)
	if err != nil {
		return nil, err
	}

	go server.Serve()
	if err := server.WaitMount(); err != nil {
		// we don't shutdown the serve loop. If the mount does
		// not succeed, the loop won't work and exit.
		return server, err
	}

	return server, nil
}

func (m *Mounter) Unmount() {
	m.stopCh <- struct{}{}
	<-m.exitCh
}

func (m *Mounter) Open() error {
	var (
		cmd  string
		args []string
	)
	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default:
		cmd = "xdg-open"
	}
	args = append(args, m.mountpoint)
	return exec.Command(cmd, args...).Start()
}
