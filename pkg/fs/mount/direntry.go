package mount

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	aliyun "github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"github.com/bububa/osssync/pkg/fs/oss"
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
	fs.NodeReleaser
	fs.NodeReaddirer
}

type DirEntry struct {
	fs.Inode
	entry       *oss.DirEntry
	mu          *sync.Mutex
	ossFS       *oss.FS
	temp        *os.File
	mnt         string
	emptyFolder bool
}

func NewDirEntry(ctx context.Context, entry *oss.DirEntry, ossFS *oss.FS, mnt string) *DirEntry {
	return &DirEntry{
		entry: entry,
		ossFS: ossFS,
		mnt:   mnt,
		mu:    new(sync.Mutex),
	}
}

func NewFile(ctx context.Context, fi *oss.FileInfo, ossFS *oss.FS, mnt string) *DirEntry {
	return &DirEntry{
		entry: oss.NewDirEntry(fi),
		ossFS: ossFS,
		mnt:   mnt,
		mu:    new(sync.Mutex),
	}
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

func (d *DirEntry) RelPath() string {
	return d.entry.Path()
}

func (d *DirEntry) SetPath(path string) {
	d.entry.SetPath(path)
}

func (d *DirEntry) FileInfo() (*oss.FileInfo, error) {
	fi, err := d.entry.Info()
	if err != nil {
		return nil, err
	}
	return fi.(*oss.FileInfo), nil
}

func (d *DirEntry) FileSize() int64 {
	fi, err := d.FileInfo()
	if err != nil || fi.IsDir() {
		return 0
	}
	return fi.Size()
}

func (d *DirEntry) ModTime() uint64 {
	fi, err := d.FileInfo()
	if err != nil {
		return 0
	}
	return uint64(fi.ModTime().Unix())
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
	if d.IsDir() {
		return nil, 0, syscall.EISDIR
	}
	file, err := d.OssFS().Open(ctx, d.RelPath())
	if err != nil {
		return nil, 0, syscall.ENOENT
	}
	fi := file.(*oss.File).Info()
	d.entry.SetInfo(fi)
	return fi, fuse.FOPEN_KEEP_CACHE, fs.OK
}

// Read supports random reading of data from the file. This gets called as many
// times as are needed to get through all the desired data len(buf) bytes at a
// time.
func (d *DirEntry) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.IsDir() {
		return nil, syscall.EISDIR
	}
	readFile := d
	if fh != nil {
		if file, ok := fh.(*DirEntry); ok {
			readFile = file
		}
	}
	if readFile.IsDir() {
		return nil, syscall.EISDIR
	}
	bs, err := d.OssFS().ReadAt(ctx, readFile.RelPath(), readFile.FileSize(), off)
	if err != nil {
		return nil, syscall.EIO
	}
	return fuse.ReadResultData(bs), fs.OK
}

func (d *DirEntry) Write(ctx context.Context, fh fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	d.mu.Lock()
	defer d.mu.Unlock()
	writeFile := d
	if fh != nil {
		if file, ok := fh.(*DirEntry); ok {
			writeFile = file
		}
	}
	if writeFile.IsDir() {
		return 0, syscall.EISDIR
	}
	fmt.Printf("write: %s, %+v, %d\n", d.Path(), fh, off)
	if writeFile.temp == nil {
		bs, err := writeFile.ossFS.ReadFile(ctx, writeFile.RelPath())
		if err != nil {
			return 0, syscall.EIO
		}
		temp, err := os.CreateTemp("", writeFile.tempFilename())
		if err != nil {
			return 0, syscall.EIO
		}
		if _, err := temp.Write(bs); err != nil {
			return 0, syscall.EIO
		}
		writeFile.temp = temp
	}
	length, err := writeFile.temp.WriteAt(data, off)
	if err != nil {
		return 0, syscall.EIO
	}
	if _, err := writeFile.temp.Seek(0, 0); err != nil {
		return 0, syscall.EIO
	}

	// writeFile.buf.Write(data)
	now := time.Now()
	writeFile.entry.UpdateModTime(now)
	return uint32(length), fs.OK
}

func (d *DirEntry) Release(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	d.mu.Lock()
	defer d.mu.Unlock()
	releaseFile := d
	if fh != nil {
		if file, ok := fh.(*DirEntry); ok {
			releaseFile = file
		}
	}
	if releaseFile.temp == nil {
		return fs.OK
	}
	_ = os.Remove(releaseFile.temp.Name())
	releaseFile.temp = nil
	return fs.OK
}

func (d *DirEntry) Flush(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.IsDir() {
		return syscall.EISDIR
	}
	flushFile := d
	if fh != nil {
		if file, ok := fh.(*DirEntry); ok {
			flushFile = file
		}
	}
	if flushFile.temp == nil {
		return fs.OK
	}
	defer func() {
		_ = os.Remove(flushFile.temp.Name())
		flushFile.temp = nil
	}()
	if err := flushFile.OssFS().Upload(ctx, flushFile.RelPath(), flushFile.temp); err != nil {
		return syscall.EIO
	}

	return fs.OK
}

func (d *DirEntry) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	d.mu.Lock()
	defer d.mu.Unlock()
	var (
		size  uint64
		t     uint64
		isDir bool
	)
	if fh != nil {
		if entry, ok := fh.(*DirEntry); ok {
			t = entry.ModTime()
			isDir = entry.IsDir()
			size = uint64(entry.FileSize())
		}
	} else {
		t = d.ModTime()
		isDir = d.IsDir()
		size = uint64(d.FileSize())
	}
	if isDir {
		out.Mode = F_DIR_RW
	} else {
		out.Mode = F_FILE_RW
	}
	out.Nlink = 1
	out.Mtime = t
	out.Atime = t
	out.Ctime = t
	out.Size = size
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
	file := NewFile(ctx, fi, d.OssFS(), d.Mountpoint())
	// file.Write(ctx, nil, bs, 0)

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
	newDir := NewDirEntry(ctx, entry, d.OssFS(), d.Mountpoint())

	child := d.NewPersistentInode(ctx, newDir, fs.StableAttr{Mode: F_DIR_RW})
	if success := d.AddChild(name, child, false); success {
		newDir.emptyFolder = true
	}
	return child, fs.OK
}

func (d *DirEntry) Rmdir(ctx context.Context, name string) syscall.Errno {
	return d.Unlink(ctx, name)
}

func (d *DirEntry) Unlink(ctx context.Context, name string) syscall.Errno {
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
	if child.IsDir() {
		if _, err := d.OssFS().RemoveAll(ctx, key); err != nil {
			return syscall.EIO
		}
	} else {
		if _, err := d.OssFS().Remove(ctx, key); err != nil {
			return syscall.EIO
		}
	}

	child.ForgetPersistent()

	return fs.OK
}

func (d *DirEntry) Truncate(size uint64) fuse.Status {
	if d.temp != nil {
		_ = os.Remove(d.temp.Name())
	}

	temp, err := os.CreateTemp("", d.tempFilename())
	if err != nil {
		return fuse.EIO
	}
	d.temp = temp

	return fuse.OK
}

func (d *DirEntry) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	d.mu.Lock()
	defer d.mu.Unlock()
	// src := filepath.Join(d.Path(), name)
	distP := newParent.EmbeddedInode()
	if distP == nil {
		distP = d.Root()
	}
	distDir := distP.Path(nil)
	// dist := filepath.Join(d.Mountpoint(), distDir, newName)
	cloudSrc := filepath.Join(d.RelPath(), name)
	cloudDist := filepath.Join(distDir, newName)
	srcNode := d.GetChild(name)
	var action func(context.Context, string, string) error
	if srcNode.IsDir() {
		if len(srcNode.Children()) > 0 {
			action = d.ossFS.RenameDir
		}
	} else {
		action = d.ossFS.Rename
	}
	// fmt.Println(src, dist, cloudSrc, cloudDist)
	if action != nil {
		if err := action(ctx, cloudSrc, cloudDist); err != nil {
			fmt.Println(err)
			return syscall.EIO
		}
	}
	if flags&fs.RENAME_EXCHANGE != 0 {
		d.ExchangeChild(name, distP, newName)
	} else {
		success := d.MvChild(name, distP, newName, false)
		if !success {
			return syscall.EPERM
		}
	}
	distNode := distP.GetChild(newName)
	if distNode != nil {
		d.rename(distNode, cloudSrc, cloudDist)
	}
	return fs.OK
}

func (d *DirEntry) rename(node *fs.Inode, src string, dist string) {
	op := node.Operations()
	if node.IsDir() {
		dir, ok := op.(*DirEntry)
		if !ok {
			return
		}
		dir.SetPath(dist)

		srcDir := filepath.ToSlash(filepath.Dir(src))
		distDir := filepath.ToSlash(filepath.Dir(dist))
		for _, ch := range node.Children() {
			if ch.IsDir() {
				chDist := strings.Replace(dir.RelPath(), srcDir, distDir, 1)
				d.rename(ch, dir.RelPath(), chDist)
			} else {
				file := ch.Operations()
				if f, ok := file.(*DirEntry); ok {
					newPath := strings.Replace(f.RelPath(), srcDir, distDir, 1)
					f.SetPath(newPath)
				}
			}
		}
	} else {
		file := node.Operations()
		if f, ok := file.(*DirEntry); ok {
			f.SetPath(dist)
		}
	}
}

func (d *DirEntry) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	d.mu.Lock()
	defer d.mu.Unlock()
	child := d.GetChild(name)
	if child == nil {
		key := filepath.Join(d.RelPath(), name)
		if obj, err := d.OssFS().Open(ctx, key); err == nil {
			file := NewFile(ctx, obj.(*oss.File).Info(), d.OssFS(), d.Mountpoint())
			child = d.NewPersistentInode(ctx, file, fs.StableAttr{Mode: F_FILE_RW})
		} else {
			iter := d.OssFS().ReadDirFile(key)
			if list, err := iter.ReadDir(ctx, 1); err == nil && len(list) > 0 {
				fi := oss.NewFileInfoWithDir(key)
				dir := NewDirEntry(ctx, oss.NewDirEntry(fi), d.OssFS(), d.Mountpoint())
				child = d.NewPersistentInode(ctx, dir, fs.StableAttr{Mode: F_DIR_RW})
			} else {
				return nil, syscall.ENOENT
			}
		}
	}
	var a fuse.AttrOut
	d.mu.Unlock()
	if errno := d.Getattr(ctx, nil, &a); errno == 0 {
		out.Attr = a.Attr
	}
	d.mu.Lock()
	return child, fs.OK
}

func (d *DirEntry) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	childrenCount := len(d.Children())
	// すでにバックエンドのファイルシステムからnodeを取得している場合
	if childrenCount > 0 {
		entries := make([]fuse.DirEntry, childrenCount)
		var i int
		for k, v := range d.Children() {
			entries[i] = fuse.DirEntry{
				Mode: v.Mode(),
				Name: k,
			}
			i += 1
		}
		return fs.NewListDirStream(entries), fs.OK
	} else if d.emptyFolder {
		return fs.NewListDirStream(nil), fs.OK
	}
	var entries []fuse.DirEntry
	iter := d.OssFS().ReadDirFile(d.RelPath())
	limit := 100
	for !iter.Completed() {
		list, err := iter.ReadDir(ctx, limit)
		if err != nil {
			return nil, syscall.ENOENT
		}
		for _, ent := range list {
			mode := F_FILE_RW
			if ent.IsDir() {
				mode = F_DIR_RW
			}
			entries = append(entries, fuse.DirEntry{
				Name: ent.Name(),
				Mode: uint32(mode),
			})
		}
	}
	return fs.NewListDirStream(entries), fs.OK
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

func (d *DirEntry) Utimens(atime *time.Time, mtime *time.Time) fuse.Status {
	// TODO metadataにatime, mimeなどを格納する？
	// https://stackoverflow.com/questions/13455168/is-there-a-way-to-touch-a-file-in-amazon-s3
	return fuse.OK
}

func (d *DirEntry) tempFilename() string {
	h := md5.New()
	h.Write([]byte(d.Path()))
	return hex.EncodeToString(h.Sum(nil))
}
