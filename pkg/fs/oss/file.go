package oss

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

var loc = time.Now().Location()

type File struct {
	bucket *oss.Bucket
	info   *FileInfo
}

func (f *File) Info() *FileInfo {
	return f.info
}

type FileInfo struct {
	obj *oss.ObjectProperties
	dir string
}

func NewFileInfoWithHeader(name string, header http.Header) *FileInfo {
	modTime, _ := time.ParseInLocation(time.RFC1123, header.Get("Last-Modified"), loc)
	size, _ := strconv.ParseInt(header.Get("Content-Length"), 10, 64)
	return &FileInfo{
		obj: &oss.ObjectProperties{
			Key:          name,
			ETag:         header.Get("Etag"),
			LastModified: modTime,
			Size:         size,
		},
	}
}

func NewFileInfo(obj *oss.ObjectProperties) *FileInfo {
	return &FileInfo{
		obj: obj,
	}
}

func NewFileInfoWithDir(dir string) *FileInfo {
	return &FileInfo{
		dir: dir,
	}
}

func (fi FileInfo) Path() string {
	if fi.obj == nil {
		return fi.dir
	}
	return fi.obj.Key
}

func (fi FileInfo) SetPath(path string) {
	if fi.obj == nil {
		fi.dir = path
		return
	}
	fi.obj.Key = path
}

func (fi FileInfo) Name() string {
	return filepath.Base(fi.Path())
}

func (fi FileInfo) Dir() string {
	return filepath.Dir(fi.Path())
}

func (fi FileInfo) ModTime() time.Time {
	if fi.obj == nil {
		return time.Time{}
	}
	return fi.obj.LastModified
}

func (fi FileInfo) UpdateModTime(t time.Time) {
	if fi.obj == nil {
		return
	}
	fi.obj.LastModified = t
}

func (fi FileInfo) Size() int64 {
	if fi.obj == nil {
		return 0
	}
	return fi.obj.Size
}

func (fi FileInfo) IsDir() bool {
	return fi.obj == nil
}

func (fi FileInfo) Mode() os.FileMode {
	mode := os.ModePerm
	if fi.IsDir() {
		return mode | os.ModeDir
	}
	return mode
}

func (fi FileInfo) Sys() any {
	return fi.obj
}

func (fi FileInfo) String() string {
	return fmt.Sprintf("path:%s, modTime:%+v, size:%d, isDir:%+v", fi.Path(), fi.ModTime(), fi.Size(), fi.IsDir())
}

func NewFile(bucket *oss.Bucket, info *FileInfo) *File {
	return &File{
		bucket: bucket,
		info:   info,
	}
}

func (f File) Stat() (fs.FileInfo, error) {
	if f.info == nil {
		return nil, fs.ErrNotExist
	}
	return f.info, nil
}

func (f File) Read(b []byte) (int, error) {
	body, err := f.bucket.GetObject(f.info.Path())
	if err != nil {
		return 0, err
	}
	defer body.Close()
	bs, err := io.ReadAll(body)
	if err != nil {
		return 0, err
	}
	return copy(b, bs), nil
}

func (f File) Close() error {
	return nil
}
