package mount

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	aliyun "github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"github.com/bububa/osssync/pkg"
	"github.com/bububa/osssync/pkg/fs/oss"
)

var _ = (directoryInterface)((*DirEntry)(nil))

type directoryInterface interface {
	fs.NodeOpener
	fs.NodeReader
	fs.NodeWriter
	fs.NodeCopyFileRanger
	fs.NodeGetattrer
	fs.NodeSetattrer
	fs.NodeGetxattrer
	fs.NodeSetxattrer
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
	entry    *oss.DirEntry
	mu       *sync.Mutex
	ossFS    *oss.FS
	temp     *os.File
	xattrs   map[string][]byte
	mnt      string
	tempFile string
	tempDir  bool
}

func NewDirEntry(ctx context.Context, entry *oss.DirEntry, ossFS *oss.FS, mnt string) *DirEntry {
	return &DirEntry{
		entry:  entry,
		ossFS:  ossFS,
		xattrs: make(map[string][]byte),
		mnt:    mnt,
		mu:     new(sync.Mutex),
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

func (d *DirEntry) SetSize(size int64) {
	d.entry.SetSize(size)
}

func (d *DirEntry) SetModTime(t time.Time) {
	d.entry.SetModTime(t)
}

func (d *DirEntry) FileInfo() (*oss.FileInfo, error) {
	fi, err := d.entry.Info()
	if err != nil {
		return nil, err
	}
	return fi.(*oss.FileInfo), nil
}

func (d *DirEntry) ETag() string {
	return d.entry.ETag()
}

func (d *DirEntry) SetETag(etag string) {
	d.entry.SetETag(etag)
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
	if d.tempFile != "" {
		return d, fuse.FOPEN_DIRECT_IO, fs.OK
	}
	file, err := d.OssFS().OpenWithEtag(ctx, d.RelPath(), d.ETag())
	if err != nil {
		if strings.Contains(err.Error(), "Not Modified") {
			if fi, err := d.FileInfo(); err == nil {
				return fi, fuse.FOPEN_KEEP_CACHE | fuse.FOPEN_CACHE_DIR, fs.OK
			}
		}
		return nil, 0, syscall.ENOENT
	}
	fi := file.(*oss.File).Info()
	d.entry.SetInfo(fi)
	return fi, fuse.FOPEN_KEEP_CACHE | fuse.FOPEN_CACHE_DIR, fs.OK
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
		} else if file, ok := fh.(*os.File); ok {
			if n, err := file.ReadAt(dest, off); err != nil {
				return nil, syscall.EIO
			} else {
				return fuse.ReadResultData(dest[:n]), fs.OK
			}
		}
	}
	if readFile.IsDir() {
		return nil, syscall.EISDIR
	}
	bs, err := d.OssFS().ReadAt(ctx, readFile.RelPath(), readFile.FileSize(), off)
	if err != nil {
		return nil, syscall.EIO
	}
	copy(dest, bs)
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
	if writeFile.temp == nil {
		if writeFile.tempFile != "" {
			temp, err := os.OpenFile(writeFile.tempFile, os.O_RDWR, os.ModePerm)
			if err != nil {
				return 0, syscall.EIO
			}
			writeFile.temp = temp
			writeFile.tempFile = ""
		} else {
			bs, err := writeFile.ossFS.ReadFile(ctx, writeFile.RelPath())
			if err != nil {
				return 0, syscall.EIO
			}
			temp, err := writeFile.CreateTemp()
			if err != nil {
				return 0, syscall.EIO
			}
			if _, err := temp.Write(bs); err != nil {
				return 0, syscall.EIO
			}
			writeFile.temp = temp
		}
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
	writeFile.SetModTime(now)
	return uint32(length), fs.OK
}

func (d *DirEntry) CopyFileRange(ctx context.Context, fhIn fs.FileHandle,
	offIn uint64, out *fs.Inode, fhOut fs.FileHandle, offOut uint64,
	len uint64, flags uint64,
) (uint32, syscall.Errno) {
	d.mu.Lock()
	defer d.mu.Unlock()
	srcFile, ok := fhIn.(*os.File)
	if !ok {
		return 0, syscall.EIO
	}
	bs := make([]byte, len)
	length, err := srcFile.ReadAt(bs, int64(offIn))
	if err != nil && errors.Is(err, io.EOF) {
		return 0, syscall.EIO
	}
	distFile, ok := fhOut.(*DirEntry)
	if !ok {
		return 0, syscall.EIO
	}
	if distFile.temp == nil {
		temp, err := distFile.CreateTemp()
		if err != nil {
			return 0, syscall.EIO
		}
		distFile.temp = temp
	}
	length, err = distFile.temp.WriteAt(bs, int64(offOut))
	if err != nil {
		return 0, syscall.EIO
	}
	if _, err := distFile.temp.Seek(0, 0); err != nil {
		return 0, syscall.EIO
	}

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

	tempfileName := releaseFile.temp.Name()
	releaseFile.temp.Close()
	_ = os.Remove(tempfileName)
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
		} else if temp, ok := fh.(*os.File); ok {
			flushFile.temp = temp
			temp.Sync()
			temp.Seek(0, 0)
		}
	}
	if flushFile.temp == nil {
		return fs.OK
	}
	defer func() {
		tempfileName := flushFile.temp.Name()
		flushFile.temp.Close()
		_ = os.Remove(tempfileName)
		flushFile.temp = nil
	}()
	if err := d.ossFS.Upload(ctx, flushFile.RelPath(), flushFile.temp); err != nil {
		return syscall.EIO
	}
	flushFile.SetETag("")
	if st, err := flushFile.temp.Stat(); err == nil {
		flushFile.SetSize(st.Size())
		flushFile.SetModTime(st.ModTime())
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
	lastModified := time.Now()
	fi := oss.NewFileInfo(&aliyun.ObjectProperties{
		Key:          filepath.Join(d.RelPath(), name),
		LastModified: lastModified,
	})
	file := NewFile(ctx, fi, d.OssFS(), d.Mountpoint())
	temp, err := file.CreateTemp()
	if err != nil {
		return nil, nil, 0, syscall.EIO
	}
	defer temp.Close()
	file.tempFile = temp.Name()

	ts := uint64(lastModified.Unix())
	out.Ctime = ts
	out.Atime = ts
	out.Mtime = ts

	if caller, ok := fuse.FromContext(ctx); ok {
		out.Uid = caller.Uid
		out.Gid = caller.Gid
	}
	child := d.NewPersistentInode(ctx, file, fs.StableAttr{Mode: F_FILE_RW})
	d.AddChild(name, child, true)
	return child, child, fuse.FOPEN_DIRECT_IO, fs.OK
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
		newDir.tempDir = true
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
	if d.tempFile != "" {
		_ = os.Remove(d.tempFile)
	}

	temp, err := d.CreateTemp()
	if err != nil {
		return fuse.EIO
	}
	d.temp = temp

	return fuse.OK
}

func (d *DirEntry) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	d.mu.Lock()
	defer d.mu.Unlock()
	distP := newParent.EmbeddedInode()
	if distP == nil {
		distP = d.Root()
	}
	distDir := distP.Path(nil)
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
	if action != nil {
		if err := action(ctx, cloudSrc, cloudDist); err != nil {
			fmt.Println("rename", err)
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
		op := distNode.Operations()
		if entry, ok := op.(*DirEntry); ok && entry.tempDir {
			entry.tempDir = false
		}
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
	} else if d.tempDir {
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

//
// func (d *DirEntry) Fsync(ctx context.Context, fh fs.FileHandle, flags uint32) syscall.Errno {
// 	d.mu.Lock()
// 	defer d.mu.Unlock()
// 	fmt.Println("fsync file:", d.Path())
// 	if file, ok := fh.(*oss.DirEntry); ok {
// 		fmt.Println("fsync", file.Name())
// 	} else if file, ok := fh.(*os.File); ok {
// 		fmt.Println("fsync osfile", file.Name())
// 	}
// 	return fs.OK
// }

// Setxattr 设置扩展属性
func (d *DirEntry) Setxattr(ctx context.Context, name string, value []byte, flags uint32) syscall.Errno {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.xattrs == nil {
		d.xattrs = make(map[string][]byte)
	}
	d.xattrs[name] = value
	return fs.OK
}

// Getxattr 获取扩展属性
func (d *DirEntry) Getxattr(ctx context.Context, name string, dist []byte) (uint32, syscall.Errno) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.xattrs == nil {
		return 0, syscall.ENODATA
	}
	val, ok := d.xattrs[name]
	if ok {
		copy(dist, val)
		return uint32(len(val)), fs.OK
	}
	// 如果属性不存在，返回 ENODATA 错误
	return 0, syscall.ENODATA
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

func (d *DirEntry) CreateTemp() (*os.File, error) {
	tmpDir := filepath.Join(os.TempDir(), pkg.AppIdentity)
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
			return nil, err
		}
	}
	return os.CreateTemp(tmpDir, d.tempFilename())
}
