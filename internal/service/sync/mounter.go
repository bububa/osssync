package sync

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/pkg/fs/mount"
	"github.com/bububa/osssync/pkg/fs/oss"
)

func Mount(ctx context.Context, cfg *config.Setting) (*mount.Mounter, error) {
	mountpoint := filepath.Join(xdg.DataHome, config.AppIdentity, "mnt", cfg.Mountpoint())
	if _, err := os.Stat(mountpoint); os.IsNotExist(err) {
		if err := os.MkdirAll(mountpoint, os.ModePerm); err != nil {
			log.Fatalln(err)
		}
	}
	clt, err := oss.NewClient(cfg.Bucket, cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return nil, err
	}
	fs := oss.NewFS(clt, oss.WithPrefix(cfg.Prefix), oss.WithLocal(cfg.Local))
	mounter := mount.NewMounter(fs, mountpoint, cfg.Mountpoint())
	go mounter.Mount(ctx)
	return mounter, nil
}
