package mount

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"github.com/bububa/osssync/pkg/fs/oss"
)

type Mounter struct {
	ossFS      *oss.FS
	stopCh     chan struct{}
	exitCh     chan struct{}
	mountpoint string
	name       string
}

func NewMounter(ossFS *oss.FS, mountpoint string, name string) *Mounter {
	return &Mounter{
		ossFS:      ossFS,
		mountpoint: mountpoint,
		name:       name,
		stopCh:     make(chan struct{}, 1),
		exitCh:     make(chan struct{}, 1),
	}
}

func (m *Mounter) Mount(ctx context.Context) error {
	root := NewFS(ctx, m.ossFS, m.mountpoint)
	srv, err := mount(m.mountpoint, root, &fs.Options{
		MountOptions: fuse.MountOptions{
			DisableXAttrs:        true,
			DirectMount:          true,
			DirectMountStrict:    true,
			SyncRead:             true,
			FsName:               fmt.Sprintf("ossfs/%s", m.name),
			Name:                 fmt.Sprintf("ossfs/%s", m.name),
			AllowOther:           false,
			RememberInodes:       true,
			IgnoreSecurityLabels: true,
			Debug:                false,
		},
	})
	if err != nil && srv == nil {
		fmt.Println(err)
		return err
	}
	go func() {
		<-m.stopCh
		if _, err := os.Stat(m.mountpoint); !os.IsNotExist(err) {
			srv.Unmount()
			srv.Wait()
		}
		os.RemoveAll(m.mountpoint)
		m.exitCh <- struct{}{}
	}()
	return nil
}

func (m *Mounter) unmount() {
}

func mount(dir string, root fs.InodeEmbedder, options *fs.Options) (*fuse.Server, error) {
	if options == nil {
		one := time.Second
		options = &fs.Options{
			EntryTimeout:    &one,
			AttrTimeout:     &one,
			NegativeTimeout: &one,
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
