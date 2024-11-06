package mount

import (
	"bytes"
	"context"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"github.com/bububa/osssync/pkg/fs/mount/storage"
	"github.com/bububa/osssync/pkg/fs/oss"
)

type fileInterface interface {
	fs.NodeOpener
	fs.NodeGetattrer
	fs.NodeReader
	fs.NodeWriter
	fs.NodeSetattrer
}

type File struct {
	fs.Inode
	info  *oss.FileInfo
	db    storage.Storage
	ossFS *oss.FS
	buf   *bytes.Buffer
	mnt   string
}

var _ = (fileInterface)((*File)(nil))

func NewFile(ctx context.Context, finfo *oss.FileInfo, fs *oss.FS, db storage.Storage, mnt string) (*File, error) {
	if _, err := db.InsertPath(ctx, finfo.Path(), finfo.ModTime()); err != nil {
		return nil, err
	}
	return &File{
		info:  finfo,
		db:    db,
		ossFS: fs,
		buf:   new(bytes.Buffer),
		mnt:   mnt,
	}, nil
}

func (f *File) Path() string {
	return filepath.Join(f.mnt, f.RelPath())
}

func (f *File) Mountpoint() string {
	return f.mnt
}

func (f *File) RelPath() string {
	return f.info.Path()
}

func (f *File) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	f.db.UpdateAccess(ctx, f.Path())
	return nil, fuse.FOPEN_KEEP_CACHE, fs.OK
}

func (f *File) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	bs, err := f.ossFS.ReadFile(ctx, f.RelPath())
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	return fuse.ReadResultData(bs), fs.OK
}

func (f *File) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	// TODO: set more strict permissions here
	modTime := uint64(f.info.ModTime().Unix())
	out.Mode = 07777
	out.Nlink = 1
	out.Mtime = modTime
	out.Atime = modTime
	out.Ctime = modTime
	out.Size = uint64(f.info.Size())
	out.SetTimeout(time.Second * 5)
	return fs.OK
}

func (f *File) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	out.Mode = in.Mode
	out.Mtime = in.Mtime
	out.Atime = in.Atime
	out.Ctime = in.Ctime
	out.Size = in.Size
	out.SetTimeout(time.Minute)

	return fs.OK
}

func (f *File) Write(ctx context.Context, fh fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	f.buf.Write(data)
	return uint32(len(data)), fs.OK
}

func (f *File) Flush(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	if f.buf.Len() == 0 {
		return fs.OK
	}

	if err := f.ossFS.Upload(ctx, f.RelPath(), f.buf); err != nil {
		return fs.ToErrno(err)
	}

	f.updateModified(ctx)
	f.buf.Reset()
	return fs.OK
}

func (f *File) updateModified(ctx context.Context) error {
	if err := f.db.UpdateModified(ctx, f.Path()); err != nil {
		return err
	}

	f.info.UpdateModTime(time.Now())

	_, parent := f.Inode.Parent()
	if parent == nil {
		return nil
	}

	if p, ok := parent.Operations().(storage.ModifiedUpdater); ok {
		return p.UpdateModified(ctx)
	}

	return nil
}
