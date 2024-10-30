package local

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"time"
)

type FileInfo struct {
	fs.FileInfo
	path    string
	modTime time.Time
	etag    string
}

func NewFileInfo(fi fs.FileInfo, opts ...Option) *FileInfo {
	ret := new(FileInfo)
	ret.FileInfo = fi
	ret.modTime = fi.ModTime()
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func (f FileInfo) Path() string {
	return f.path
}

func (f FileInfo) Dir() string {
	return filepath.Dir(f.path)
}

func (f FileInfo) ETag() string {
	return f.etag
}

func (f FileInfo) ModTime() time.Time {
	return f.modTime
}

func (f FileInfo) String() string {
	return fmt.Sprintf("path:%s, modTime:%+v, size:%d, isDir:%+v", f.Path(), f.ModTime(), f.Size(), f.IsDir())
}
