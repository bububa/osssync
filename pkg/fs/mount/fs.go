package mount

import (
	"context"
	"fmt"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"

	"github.com/bububa/osssync/pkg/fs/mount/storage"
	"github.com/bububa/osssync/pkg/fs/oss"
)

var _ = (rootInterface)((*FS)(nil))

type rootInterface interface {
	iFS
	fs.NodeOnAdder
}

type iFS interface {
	fs.InodeEmbedder
	fs.NodeCreater
	fs.NodeMkdirer
	fs.NodeRenamer
	fs.NodeGetattrer
	fs.NodeUnlinker
	storage.ModifiedUpdater
}

type FS struct {
	iFS
	ossFS *oss.FS
	db    storage.Storage
	mnt   string
}

func NewFS(ctx context.Context, fs *oss.FS, mnt string, db storage.Storage) (*FS, error) {
	dir, err := NewDirEntry(ctx, oss.NewDirEntry(oss.NewFileInfoWithDir("")), fs, db, mnt)
	if err != nil {
		return nil, err
	}

	return &FS{
		iFS:   dir,
		ossFS: fs,
		db:    db,
		mnt:   mnt,
	}, nil
}

func (r *FS) Iter(ctx context.Context, reader *oss.ReadDirFile, p *fs.Inode) {
	limit := 100
	for !reader.Completed() {
		entries, err := reader.ReadDir(ctx, limit)
		if err != nil {
			fmt.Println(err)
			continue
		}
		for _, entry := range entries {
			entry := entry.(*oss.DirEntry)
			if p == nil {
				p = r.EmbeddedInode()
			}
			if entry.IsDir() {
				child := p.GetChild(entry.Path())
				if child == nil {
					dir, err := NewDirEntry(ctx, entry, r.ossFS, r.db, r.mnt)
					if err != nil {
						continue
					}
					child = p.NewPersistentInode(ctx, dir, fs.StableAttr{Mode: syscall.S_IFDIR})
					p.AddChild(entry.Path(), child, false)
				}
				p = child
				r.Iter(ctx, oss.NewReadDirFile(r.ossFS, entry.Path()), p)
			} else {
				fi, err := entry.Info()
				if err != nil {
					continue
				}
				file, err := NewFile(ctx, fi.(*oss.FileInfo), r.ossFS, r.db, r.mnt)
				if err != nil {
					continue
				}
				// Create the file. The Inode must be persistent,
				// because its life time is not under control of the
				// kernel.
				child := p.NewPersistentInode(ctx, file, fs.StableAttr{})

				// And add it
				p.AddChild(entry.Name(), child, false)
			}
		}
	}
}

func (r *FS) OnAdd(ctx context.Context) {
	r.Iter(ctx, oss.NewReadDirFile(r.ossFS, ""), nil)
}
