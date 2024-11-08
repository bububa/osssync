package mount

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	fs.NodeOpener
	fs.NodeReader
	fs.NodeWriter
	fs.NodeGetattrer
	fs.NodeSetattrer
	fs.NodeCreater
	fs.NodeMkdirer
	fs.NodeRmdirer
	fs.NodeUnlinker
	fs.NodeAccesser
	fs.NodeRenamer
	fs.NodeLookuper
	storage.ModifiedUpdater
}

type DirEntry struct {
	fs.Inode
	entry *oss.DirEntry
	buf   *bytes.Buffer
	mu    *sync.Mutex
	ossFS *oss.FS
	db    storage.Storage
	mnt   string
}

func NewDirEntry(ctx context.Context, entry *oss.DirEntry, ossFS *oss.FS, db storage.Storage, mnt string) (*DirEntry, error) {
	fi, err := entry.Info()
	if err != nil {
		return nil, err
	}
	if _, err := db.InsertPath(ctx, entry.Path(), fi.ModTime()); err != nil {
		return nil, err
	}

	return &DirEntry{
		entry: entry,
		ossFS: ossFS,
		db:    db,
		mnt:   mnt,
		buf:   new(bytes.Buffer),
		mu:    new(sync.Mutex),
	}, nil
}

func (d *DirEntry) Path() string {
	return filepath.Join(d.Mountpoint(), d.RelPath())
}

func (d *DirEntry) Mountpoint() string {
	return d.mnt
}

func (d *DirEntry) OssFS() *oss.FS {
	return d.ossFS
}

func (d *DirEntry) DB() storage.Storage {
	return d.db
}

func (d *DirEntry) RelPath() string {
	return d.entry.Path()
}

func (d *DirEntry) FileInfo() (*oss.FileInfo, error) {
	fi, err := d.entry.Info()
	if err != nil {
		return nil, err
	}
	return fi.(*oss.FileInfo), nil
}

func (d *DirEntry) IsDir() bool {
	return d.EmbeddedInode().IsDir()
}

func (d *DirEntry) Access(ctx context.Context, input uint32) (errno syscall.Errno) {
	return fs.OK
}

func (d *DirEntry) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.IsDir() {
		return nil, 0, syscall.ENOTDIR
	}
	fmt.Println(d.Path())
	d.DB().UpdateAccess(ctx, d.Path())
	return nil, fuse.FOPEN_KEEP_CACHE, fs.OK
}

func (d *DirEntry) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return nil, syscall.EPERM
	if !d.IsDir() {
		return nil, fs.ToErrno(errors.New("read operation not permitted for dir"))
	}
	bs, err := d.OssFS().ReadFile(ctx, d.RelPath())
	if err != nil {
		return nil, syscall.EREMOTE
	}
	return fuse.ReadResultData(bs), fs.OK
}

func (d *DirEntry) Write(ctx context.Context, fh fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.buf.Write(data)
	fmt.Printf("writedir:%s, %d, %+v\n", d.Path(), off, fh)
	return uint32(len(data)), fs.OK
}

func (d *DirEntry) Flush(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	d.mu.Lock()
	defer d.mu.Unlock()
	return syscall.EPERM
	if d.buf.Len() == 0 {
		return fs.OK
	}

	if err := d.OssFS().Upload(ctx, d.RelPath(), d.buf); err != nil {
		return syscall.EREMOTE
	}

	// f.updateModified(ctx)
	d.buf.Reset()
	return fs.OK
}

func (d *DirEntry) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	d.mu.Lock()
	defer d.mu.Unlock()
	var t uint64
	if fi, err := d.FileInfo(); err == nil {
		t = uint64(fi.ModTime().Unix())
	}
	if d.IsDir() {
		out.Mode = F_DIR_RW
	} else {
		out.Mode = F_FILE_RW
	}
	out.Nlink = 1
	out.Mtime = t
	out.Atime = t
	out.Ctime = t
	out.SetTimeout(time.Second * 5)
	return fs.OK
}

func (d *DirEntry) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	d.mu.Lock()
	defer d.mu.Unlock()
	out.Mode = in.Mode
	out.Mtime = in.Mtime
	out.Atime = in.Atime
	out.Ctime = in.Ctime
	out.Size = in.Size
	out.SetTimeout(time.Minute)

	return fs.OK
}

func (d *DirEntry) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	d.mu.Lock()
	defer d.mu.Unlock()
	path := filepath.Join(d.Path(), name)
	fmt.Println("create:", path)
	// flags = flags &^ syscall.O_APPEND
	// fd, err := syscall.Open(path, int(flags)|os.O_CREATE, mode)
	// if err != nil {
	// 	return nil, nil, 0, fs.ToErrno(err)
	// }
	// defer syscall.Close(fd)
	// st := syscall.Stat_t{}
	// if err := syscall.Fstat(fd, &st); err != nil {
	// 	return nil, nil, 0, fs.ToErrno(err)
	// }
	// out.FromStat(&st)
	// fmt.Println("get file bytes", len(bs))
	// if err != nil {
	// 	return nil, nil, 0, fs.ToErrno(err)
	// }
	// size := st.Size
	lastModified := time.Now()
	fi := oss.NewFileInfo(&aliyun.ObjectProperties{
		Key:          filepath.Join(d.RelPath(), name),
		LastModified: lastModified,
		// Size:         size,
	})
	file, err := NewFile(ctx, fi, d.OssFS(), d.DB(), d.Mountpoint())
	// file.Write(ctx, nil, bs, 0)
	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	child := d.NewPersistentInode(ctx, file, fs.StableAttr{Mode: F_FILE_RW})
	d.AddChild(name, child, true)
	fmt.Println("create end:", path)
	return child, file, 0, fs.OK
}

func (d *DirEntry) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.IsDir() {
		return nil, syscall.ENOTDIR
	}
	path := filepath.Join(d.RelPath(), name)
	fi := oss.NewFileInfoWithDir(path)
	entry := oss.NewDirEntry(fi)
	newDir, err := NewDirEntry(ctx, entry, d.OssFS(), d.DB(), d.Mountpoint())
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	child := d.NewPersistentInode(ctx, newDir, fs.StableAttr{Mode: F_DIR_RW})
	d.AddChild(name, child, true)
	return child, fs.OK
}

func (d *DirEntry) Rmdir(ctx context.Context, name string) syscall.Errno {
	fmt.Println("rmdir")
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.IsDir() {
		return syscall.ENOTDIR
	}
	child := d.GetChild(name)
	if child == nil {
		return fs.OK
	}
	key := filepath.Join(d.RelPath(), name)
	if _, err := d.OssFS().Remove(ctx, key); err != nil {
		return syscall.EREMOTE
	}

	child.ForgetPersistent()

	return fs.OK
}

func (d *DirEntry) Unlink(ctx context.Context, name string) syscall.Errno {
	fmt.Println("unlink")
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.IsDir() {
		return syscall.ENOTDIR
	}
	child := d.GetChild(name)
	if child == nil {
		return fs.OK
	}

	// TODO: handle unlink of directories
	key := filepath.Join(d.RelPath(), name)
	if _, err := d.OssFS().Remove(ctx, key); err != nil {
		return syscall.EREMOTE
	}

	child.ForgetPersistent()

	return fs.OK
}

func (d *DirEntry) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	d.mu.Lock()
	defer d.mu.Unlock()
	src := filepath.Join(d.Path(), name)
	distP := newParent.EmbeddedInode()
	if distP == nil {
		distP = d.Root()
	}
	distDir := distP.Path(nil)
	dist := filepath.Join(d.Mountpoint(), distDir, newName)
	cloudSrc := filepath.Join(d.RelPath(), name)
	cloudDist := filepath.Join(distDir, newName)
	srcNode := d.GetChild(name)
	var action func(context.Context, string, string) error
	if srcNode.IsDir() {
		action = d.ossFS.RenameDir
	} else {
		action = d.ossFS.Rename
	}
	// fmt.Println(src, dist, cloudSrc, cloudDist)
	if err := action(ctx, cloudSrc, cloudDist); err != nil {
		fmt.Println(err)
		return syscall.EREMOTE
	}
	fmt.Println("rename", name, ", ", newName, ", ", distP, ", ", src, dist, cloudSrc, cloudDist)
	if flags&fs.RENAME_EXCHANGE != 0 {
		d.ExchangeChild(name, distP, newName)
	} else {
		success := d.MvChild(name, distP, newName, false)
		if !success {
			return syscall.EPERM
		}
		fmt.Println(distP, ", ", d.Root())
	}
	d.rename(srcNode, cloudSrc, cloudDist)
	return fs.OK
}

func (d *DirEntry) rename(node *fs.Inode, src string, dist string) {
	op := node.Operations()
	if node.IsDir() {
		dir, ok := op.(*DirEntry)
		if !ok {
			return
		}
		if fi, err := dir.FileInfo(); err == nil {
			fi.SetPath(dist)
		}

		srcDir := filepath.ToSlash(filepath.Dir(src))
		distDir := filepath.ToSlash(filepath.Dir(dist))
		for _, ch := range node.Children() {
			if ch.IsDir() {
				chDist := strings.Replace(dir.RelPath(), srcDir, distDir, 1)
				d.rename(ch, dir.RelPath(), chDist)
			} else {
				file := ch.Operations()
				if f, ok := file.(*File); ok {
					f.Info().SetPath(strings.Replace(f.Info().Path(), srcDir, distDir, 1))
				}
			}
		}
	} else {
		file := node.Operations()
		if f, ok := file.(*File); ok {
			f.Info().SetPath(dist)
		}
	}
}

func (d *DirEntry) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	d.mu.Lock()
	defer d.mu.Unlock()
	child := d.GetChild(name)
	if child == nil {
		return nil, syscall.ENOENT
	}
	var a fuse.AttrOut
	d.mu.Unlock()
	if errno := d.Getattr(ctx, nil, &a); errno == 0 {
		out.Attr = a.Attr
	}
	d.mu.Lock()
	return child, fs.OK
}

// preserveOwner sets uid and gid of `path` according to the caller information
// in `ctx`.
func (d *DirEntry) preserveOwner(ctx context.Context, path string) error {
	if os.Getuid() != 0 {
		return nil
	}
	caller, ok := fuse.FromContext(ctx)
	if !ok {
		return nil
	}
	return syscall.Lchown(path, int(caller.Uid), int(caller.Gid))
}

func (d *DirEntry) UpdateModified(ctx context.Context) error {
	if err := d.DB().UpdateModified(ctx, d.Path()); err != nil {
		return err
	}

	if fi, err := d.FileInfo(); err == nil {
		fi.UpdateModTime(time.Now())
	}

	_, parent := d.Parent()
	if parent == nil {
		return nil
	}

	if p, ok := parent.Operations().(storage.ModifiedUpdater); ok {
		return p.UpdateModified(ctx)
	}

	return nil
}
