package mount

import (
	"context"
	"fmt"

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
	fs.NodeOpener
	fs.NodeReader
	fs.NodeWriter
	fs.NodeMkdirer
	fs.NodeRmdirer
	fs.NodeRenamer
	fs.NodeGetattrer
	fs.NodeSetattrer
	fs.NodeUnlinker
	fs.NodeLookuper
	storage.ModifiedUpdater
}

type FS struct {
	iFS
	ossFS *oss.FS
	db    storage.Storage
	mnt   string
}

func NewFS(ctx context.Context, fs *oss.FS, mnt string, db storage.Storage) (*FS, error) {
	ret := &FS{
		ossFS: fs,
		db:    db,
		mnt:   mnt,
	}
	dir, err := NewDirEntry(ctx, oss.NewDirEntry(oss.NewFileInfoWithDir("")), fs, db, mnt)
	if err != nil {
		return nil, err
	}
	ret.iFS = dir
	return ret, nil
}

func (f *FS) Mountpoint() string {
	return f.mnt
}

func (f *FS) OssFS() *oss.FS {
	return f.ossFS
}

func (f *FS) DB() storage.Storage {
	return f.db
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
				child := p.GetChild(entry.Name())
				if child == nil {
					dir, err := NewDirEntry(ctx, entry, r.OssFS(), r.DB(), r.Mountpoint())
					if err != nil {
						continue
					}
					child = p.NewPersistentInode(ctx, dir, fs.StableAttr{Mode: F_DIR_RW})
					p.AddChild(entry.Name(), child, true)
					op := child.Operations()
					if _, ok := op.(*DirEntry); ok {
						fmt.Println("isDirEntry", entry.Name())
					}
				}
				p = child
				r.Iter(ctx, oss.NewReadDirFile(r.ossFS, entry.Path()), p)
			} else {
				fi, err := entry.Info()
				if err != nil {
					continue
				}
				file, err := NewFile(ctx, fi.(*oss.FileInfo), r.OssFS(), r.DB(), r.Mountpoint())
				if err != nil {
					continue
				}
				// Create the file. The Inode must be persistent,
				// because its life time is not under control of the
				// kernel.
				child := p.NewPersistentInode(ctx, file, fs.StableAttr{Mode: F_FILE_RW})
				op := child.Operations()
				if _, ok := op.(*File); ok {
					fmt.Println("isFile", entry.Name())
				}

				// And add it
				p.AddChild(entry.Name(), child, true)
			}
		}
	}
}

func (r *FS) OnAdd(ctx context.Context) {
	r.Iter(ctx, oss.NewReadDirFile(r.ossFS, ""), nil)
}
