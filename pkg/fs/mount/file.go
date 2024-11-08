package mount

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"sync"
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
	fs.NodeSetattrer
	fs.NodeReader
	fs.NodeWriter
}

type File struct {
	fs.Inode
	ossFS *oss.FS
	db    storage.Storage
	mu    *sync.Mutex
	info  *oss.FileInfo
	buf   *bytes.Buffer
	mnt   string
}

var _ = (fileInterface)((*File)(nil))

func NewFile(ctx context.Context, finfo *oss.FileInfo, ossFS *oss.FS, db storage.Storage, mnt string) (*File, error) {
	if _, err := db.InsertPath(ctx, finfo.Path(), finfo.ModTime()); err != nil {
		return nil, err
	}
	return &File{
		info:  finfo,
		ossFS: ossFS,
		db:    db,
		mnt:   mnt,
		buf:   new(bytes.Buffer),
		mu:    new(sync.Mutex),
	}, nil
}

func (f *File) Path() string {
	return filepath.Join(f.mnt, f.RelPath())
}

func (f *File) RelPath() string {
	return f.info.Path()
}

func (f *File) Mountpoint() string {
	return f.mnt
}

func (f *File) OssFS() *oss.FS {
	return f.ossFS
}

func (f *File) DB() storage.Storage {
	return f.db
}

func (f *File) Info() *oss.FileInfo {
	return f.info
}

func (f *File) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if fi, err := f.OssFS().Open(ctx, f.RelPath()); err == nil {
		f.info = fi.(*oss.File).Info()
	} else {
		return nil, 0, syscall.ENOENT
	}
	f.DB().UpdateAccess(ctx, f.Path())
	return f.info, fuse.FOPEN_KEEP_CACHE, fs.OK
}

func (f *File) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	f.mu.Lock()
	defer f.mu.Unlock()
	bs, err := f.OssFS().ReadAt(ctx, f.RelPath(), f.info.Size(), off)
	if err != nil {
		return nil, syscall.EREMOTE
	}
	return fuse.ReadResultData(bs), fs.OK
}

func (f *File) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()
	modTime := uint64(f.info.ModTime().Unix())
	out.Mode = F_FILE_RW
	out.Nlink = 1
	out.Mtime = modTime
	out.Atime = modTime
	out.Ctime = modTime
	out.Size = uint64(f.info.Size())
	out.SetTimeout(time.Second * 5)
	return fs.OK
}

func (f *File) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()
	out.Mode = in.Mode
	out.Mtime = in.Mtime
	out.Atime = in.Atime
	out.Ctime = in.Ctime
	out.Size = in.Size
	out.SetTimeout(time.Minute)

	return fs.OK
}

func (f *File) Write(ctx context.Context, fh fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	fmt.Println("fs write", f.Path())
	f.mu.Lock()
	defer f.mu.Unlock()
	if fh != nil {
		if file, ok := fh.(*File); ok {
			fmt.Println("oss file write")
			f.mu.Unlock()
			written, errno = file.Write(ctx, nil, data, off)
			f.mu.Lock()
			return
		}
	}
	fmt.Printf("write: %s, %+v, %d\n", f.Path(), fh, off)
	f.buf.Write(data)
	return uint32(len(data)), fs.OK
}

func (f *File) Flush(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()
	if fh != nil {
		if file, ok := fh.(*File); ok {
			fmt.Println("oss file flush")
			f.mu.Unlock()
			errno := file.Flush(ctx, nil)
			f.mu.Lock()
			return errno
		}
	}
	fmt.Println("flush", f.Path())
	if f.buf.Len() == 0 {
		return fs.OK
	}

	if err := f.OssFS().Upload(ctx, f.RelPath(), f.buf); err != nil {
		return syscall.EREMOTE
	}
	fmt.Println("flush uploaded", f.Path())

	f.updateModified(ctx)
	f.buf.Reset()
	return fs.OK
}

// func (f *File) GetLk(owner uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) (code fuse.Status) {
// 	f.mu.Lock()
// 	defer f.mu.Unlock()
// 	return f.getLk(owner, lk, flags, out)
// }
//
// func (f *File) SetLk(owner uint64, lk *fuse.FileLock, flags uint32) (code fuse.Status) {
// 	f.mu.Lock()
// 	defer f.mu.Unlock()
// 	return f.setLock(owner, lk, flags, false)
// }
//
// func (f *File) SetLkw(owner uint64, lk *fuse.FileLock, flags uint32) (code fuse.Status) {
// 	f.mu.Lock()
// 	defer f.mu.Unlock()
// 	return f.setLock(owner, lk, flags, true)
// }
//
// func (f *File) getLk(owner uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) (code fuse.Status) {
// 	flk := syscall.Flock_t{}
// 	lk.ToFlockT(&flk)
// 	code = fuse.ToStatus(syscall.FcntlFlock(f.File.Fd(), F_OFD_GETLK, &flk))
// 	out.FromFlockT(&flk)
// 	return
// }
//
// func (f *File) setLock(owner uint64, lk *fuse.FileLock, flags uint32, blocking bool) (code fuse.Status) {
// 	if (flags & fuse.FUSE_LK_FLOCK) != 0 {
// 		var op int
// 		switch lk.Typ {
// 		case syscall.F_RDLCK:
// 			op = syscall.LOCK_SH
// 		case syscall.F_WRLCK:
// 			op = syscall.LOCK_EX
// 		case syscall.F_UNLCK:
// 			op = syscall.LOCK_UN
// 		default:
// 			return fuse.EINVAL
// 		}
// 		if !blocking {
// 			op |= syscall.LOCK_NB
// 		}
// 		return fuse.ToStatus(syscall.Flock(int(f.EmbeddedInode().Fd()), op))
// 	} else {
// 		flk := syscall.Flock_t{}
// 		lk.ToFlockT(&flk)
// 		var op int
// 		if blocking {
// 			op = F_OFD_SETLKW
// 		} else {
// 			op = F_OFD_SETLK
// 		}
// 		return fuse.ToStatus(syscall.FcntlFlock(f.File.Fd(), op, &flk))
// 	}
// }

func (f *File) updateModified(ctx context.Context) error {
	if err := f.DB().UpdateModified(ctx, f.Path()); err != nil {
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
