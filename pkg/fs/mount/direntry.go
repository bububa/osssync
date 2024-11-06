package mount

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	aliyun "github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/bububa/osssync/pkg/fs/mount/storage"
	"github.com/bububa/osssync/pkg/fs/oss"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

var _ = (directoryInterface)((*DirEntry)(nil))

type directoryInterface interface {
	fs.NodeGetattrer
	fs.NodeCreater
	fs.NodeMkdirer
	fs.NodeUnlinker
	fs.NodeRenamer
	storage.ModifiedUpdater
}

type DirEntry struct {
	fs.Inode
	entry *oss.DirEntry
	ossFS *oss.FS
	db    storage.Storage
	mnt   string
}

func NewDirEntry(ctx context.Context, entry *oss.DirEntry, fs *oss.FS, db storage.Storage, mnt string) (*DirEntry, error) {
	fi, err := entry.Info()
	if err != nil {
		return nil, err
	}
	if _, err := db.InsertPath(ctx, entry.Path(), fi.ModTime()); err != nil {
		return nil, err
	}

	return &DirEntry{
		entry: entry,
		ossFS: fs,
		db:    db,
		mnt:   mnt,
	}, nil
}

func (d *DirEntry) Path() string {
	return filepath.Join(d.mnt, d.RelPath())
}

func (d *DirEntry) Mountpoint() string {
	return d.mnt
}

func (d *DirEntry) RelPath() string {
	return d.entry.Path()
}

func (d *DirEntry) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	var t uint64
	if fi, err := d.entry.Info(); err == nil {
		t = uint64(fi.ModTime().Unix())
	}
	out.Mode = 07777
	out.Nlink = 1
	out.Mtime = t
	out.Atime = t
	out.Ctime = t
	out.SetTimeout(time.Second * 5)
	return fs.OK
}

func (d *DirEntry) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	path := filepath.Join(d.Path(), name)
	flags = flags &^ syscall.O_APPEND
	fd, err := syscall.Open(path, int(flags)|os.O_CREATE, mode)
	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}
	st := syscall.Stat_t{}
	if err := syscall.Fstat(fd, &st); err != nil {
		syscall.Close(fd)
		return nil, nil, 0, fs.ToErrno(err)
	}
	out.FromStat(&st)
	size := int64(0)
	lastModified := time.Now()
	fi := oss.NewFileInfo(&aliyun.ObjectProperties{
		Key:          d.RelPath(),
		LastModified: lastModified,
		Size:         size,
	})
	file, err := NewFile(ctx, fi, d.ossFS, d.db, d.mnt)
	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	child := d.NewPersistentInode(ctx, file, fs.StableAttr{})
	d.AddChild(name, child, true)
	return child, nil, 0, fs.OK
}

func (d *DirEntry) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	path := filepath.Join(d.RelPath(), name)
	fi := oss.NewFileInfoWithDir(path)
	entry := oss.NewDirEntry(fi)
	newDir, err := NewDirEntry(ctx, entry, d.ossFS, d.db, d.mnt)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	child := d.NewPersistentInode(ctx, newDir, fs.StableAttr{Mode: syscall.S_IFDIR})
	d.AddChild(name, child, true)
	return child, fs.OK
}

func (d *DirEntry) Unlink(ctx context.Context, name string) syscall.Errno {
	child := d.Inode.GetChild(name)
	if child == nil {
		return fs.OK
	}

	// TODO: handle unlink of directories
	key := filepath.Join(d.RelPath(), name)
	if _, err := d.ossFS.Remove(ctx, key); err != nil {
		return fs.ToErrno(err)
	}

	child.ForgetPersistent()

	return fs.OK
}

func (d *DirEntry) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	src := filepath.Join(d.Path(), name)
	dist := filepath.Join(d.Path(), newParent.EmbeddedInode().Path(nil), newName)
	var action func(context.Context, string, string) error
	if st, err := os.Stat(src); err != nil {
		return fs.ToErrno(err)
	} else if st.IsDir() {
		action = d.ossFS.RenameDir
	} else {
		action = d.ossFS.Rename
	}
	cloudSrc := filepath.Join(d.RelPath(), name)
	cloudDist := filepath.Join(d.RelPath(), newParent.EmbeddedInode().Path(nil), newName)
	if err := action(ctx, cloudSrc, cloudDist); err != nil {
		return fs.ToErrno(err)
	}
	err := syscall.Rename(src, dist)
	fmt.Println(src, dist, cloudSrc, cloudDist)
	return fs.ToErrno(err)
}

func (d *DirEntry) UpdateModified(ctx context.Context) error {
	if err := d.db.UpdateModified(ctx, d.Path()); err != nil {
		return err
	}

	if fi, err := d.entry.Info(); err == nil {
		fi.(*oss.FileInfo).UpdateModTime(time.Now())
	}

	_, parent := d.Inode.Parent()
	if parent == nil {
		return nil
	}

	if p, ok := parent.Operations().(storage.ModifiedUpdater); ok {
		return p.UpdateModified(ctx)
	}

	return nil
}
