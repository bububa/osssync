package mount

import (
	"context"
	"fmt"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"github.com/bububa/osssync/pkg/fs/oss"
)

const (
	blockSize   = uint64(4096)
	totalBlocks = uint64(274877906944) // 1PB / blockSize
	inodes      = uint64(1000000000)
	ioSize      = uint32(1048576) // 1MB
)

var _ = (rootInterface)((*FS)(nil))

type rootInterface interface {
	iFS
	fs.NodeOnAdder
	fs.NodeStatfser
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
}

type FS struct {
	iFS
	ossFS *oss.FS
	mnt   string
}

func NewFS(ctx context.Context, fs *oss.FS, mnt string) *FS {
	return &FS{
		iFS:   NewDirEntry(ctx, oss.NewDirEntry(oss.NewFileInfoWithDir("")), fs, mnt),
		ossFS: fs,
		mnt:   mnt,
	}
}

func (f *FS) Mountpoint() string {
	return f.mnt
}

func (f *FS) OssFS() *oss.FS {
	return f.ossFS
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
					dir := NewDirEntry(ctx, entry, r.OssFS(), r.Mountpoint())
					child = p.NewPersistentInode(ctx, dir, fs.StableAttr{Mode: F_DIR_RW})
					p.AddChild(entry.Name(), child, true)
				}
				p = child
				r.Iter(ctx, oss.NewReadDirFile(r.ossFS, entry.Path()), p)
			} else {
				file := NewDirEntry(ctx, entry, r.OssFS(), r.Mountpoint())
				// Create the file. The Inode must be persistent,
				// because its life time is not under control of the
				// kernel.
				child := p.NewPersistentInode(ctx, file, fs.StableAttr{Mode: F_FILE_RW})
				// And add it
				p.AddChild(entry.Name(), child, true)
			}
		}
	}
}

func (r *FS) OnAdd(ctx context.Context) {
	r.Iter(ctx, oss.NewReadDirFile(r.ossFS, ""), nil)
}

// Statfs returns a constant (faked) set of details describing a very large
// file system.
func (r *FS) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	out.Blocks = blockSize
	out.Bfree = totalBlocks
	out.Bavail = totalBlocks
	out.Files = inodes
	out.Ffree = inodes
	out.Bsize = ioSize
	// NameLen uint32
	// Frsize  uint32
	// Padding uint32
	// Spare   [6]uint32
	return fs.OK
}
