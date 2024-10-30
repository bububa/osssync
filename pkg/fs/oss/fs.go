package oss

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	local "github.com/bububa/osssync/pkg/fs/local"
)

const (
	MaxKeys         = 1000
	MinBigFile      = 500 << 20
	MaxParts        = 10000
	MinParts        = 1000
	DefaultPartSize = 500 << 10
)

func clearDirPath(dir string) string {
	return path.Clean(filepath.ToSlash(dir)) + "/"
}

type FS struct {
	clt      *Client
	listener *MultiProgressListener
	local    string
	prefix   string
}

func NewFS(clt *Client, opts ...Option) *FS {
	ret := &FS{
		clt:      clt,
		listener: NewMultiProgressListener(),
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

func (f *FS) Open(name string) (fs.File, error) {
	header, err := f.clt.bucket.GetObjectDetailedMeta(name)
	if err != nil {
		if e, ok := err.(oss.ServiceError); ok && e.Code == "NoSuchKey" {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	info := NewFileInfoWithHeader(name, header)
	return NewFile(f.clt.bucket, info), nil
}

func (f *FS) ReadFile(name string) ([]byte, error) {
	body, err := f.clt.bucket.GetObject(name)
	if err != nil {
		if e, ok := err.(oss.ServiceError); ok && e.Code == "NoSuchKey" {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	defer body.Close()
	return io.ReadAll(body)
}

func (f *FS) Stat(name string) (fs.FileInfo, error) {
	file, err := f.Open(name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	return file.Stat()
}

func (f *FS) Exists(key string, etag string) bool {
	var options []oss.Option
	if etag != "" {
		options = []oss.Option{oss.IfNoneMatch(etag)}
	}
	ret, _ := f.clt.bucket.IsObjectExist(key, options...)
	return ret
}

func (f *FS) Upload(localFile *local.FileInfo) error {
	remotePath, err := f.RemotePathFromLocalFile(localFile)
	if err != nil {
		return err
	}

	if s, err := f.Stat(remotePath); err == nil {
		if s.ModTime().After(localFile.ModTime()) {
			return nil
		}
	}
	opts := []oss.Option{
		oss.ACL(oss.ACLPrivate),
		oss.Progress(f.listener.UploadListener(localFile.Path(), remotePath)),
	}
	if localFile.Size() >= MinBigFile {
		cpDir := f.uploadCheckoutPointPath(localFile)
		opts = append(opts, oss.Routines(3), oss.Checkpoint(true, cpDir))
		return f.clt.bucket.UploadFile(remotePath, localFile.Path(), calPartSize(localFile.Size()), opts...)
	}
	return f.clt.bucket.PutObjectFromFile(remotePath, localFile.Path(), opts...)
}

func (f *FS) Remove(keys ...string) ([]string, error) {
	res, err := f.clt.bucket.DeleteObjects(keys)
	if err != nil {
		return nil, err
	}
	mp := make(map[string]struct{}, len(res.DeletedObjects))
	for _, v := range res.DeletedObjects {
		f.listener.RemoveListener(v).ProgressChanged(&oss.ProgressEvent{
			EventType: oss.TransferCompletedEvent,
		})
	}
	for _, v := range keys {
		if _, ok := mp[v]; !ok {
			f.listener.RemoveListener(v).ProgressChanged(&oss.ProgressEvent{
				EventType: oss.TransferFailedEvent,
			})
		}
	}
	return res.DeletedObjects, nil
}

func (f *FS) RemoveAll(dir string) ([]string, error) {
	var ret []string
	_, err := f.clt.list(dir, func(list []oss.ObjectProperties) error {
		keys := make([]string, 0, len(list))
		for _, v := range list {
			keys = append(keys, v.Key)
		}
		if list, err := f.Remove(keys...); err != nil {
			return err
		} else {
			ret = append(ret, list...)
		}
		return nil
	})
	return ret, err
}

func (f *FS) Rename(src string, dist string) error {
	if err := f.Copy(src, dist); err != nil {
		return err
	}
	if _, err := f.Remove(src); err != nil {
		return err
	}
	return nil
}

func (f *FS) Copy(src string, dist string) error {
	_, err := f.clt.bucket.CopyObject(src, dist, oss.Progress(f.listener.CopyListener(src, dist)))
	return err
}

func (f *FS) Download(remotePath string, localPath string) error {
	fn := filepath.Base(remotePath)
	localFile := filepath.Join(localPath, fn)
	cpDir := f.downloadCheckoutPointPath(localPath, fn)
	return f.clt.bucket.DownloadFile(remotePath, localFile, DefaultPartSize, oss.Checkpoint(true, cpDir), oss.Progress(f.listener.DownloadListener(remotePath, localFile)))
}

func (f *FS) RemotePathFromLocalFile(localFile *local.FileInfo) (string, error) {
	if !strings.HasPrefix(localFile.Path(), f.local) {
		return "", errors.New("invalid local file, file not inside local setting path")
	}
	return strings.Replace(localFile.Path(), f.local, f.prefix, 1), nil
}

func (f *FS) uploadCheckoutPointPath(localFile *local.FileInfo) string {
	return filepath.Join(filepath.Dir(localFile.Path()), ".osssync-upload", localFile.Name())
}

func (f *FS) downloadCheckoutPointPath(path string, name string) string {
	return filepath.Join(filepath.Dir(path), ".osssync-download", name)
}

func (f *FS) Events() <-chan ProgressEvent {
	return f.listener.Events()
}

func (f *FS) Close() {
	f.listener.Close()
}

func calPartSize(size int64) int64 {
	if size/DefaultPartSize > MaxParts {
		return size / MinParts
	}
	return DefaultPartSize
}

type ReadDirFS struct {
	FS
}

func NewReadDirFS(fs *FS, name string) *ReadDirFS {
	ret := new(ReadDirFS)
	ret.clt = fs.clt
	return ret
}

func (f *ReadDirFS) ReadDir(name string) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry
	prefix := oss.Prefix(clearDirPath(name))
	continuationToken := oss.ContinuationToken("")
	startAfter := oss.StartAfter("")
	for {
		res, err := f.clt.bucket.ListObjectsV2(prefix, startAfter, continuationToken, oss.Delimiter("/"), oss.MaxKeys(MaxKeys))
		if err != nil {
			return entries, err
		}
		for _, obj := range res.Objects {
			entry := NewDirEntry(NewFileInfo(&obj))
			entries = append(entries, entry)
		}
		for _, dir := range res.CommonPrefixes {
			entry := NewDirEntry(NewFileInfoWithDir(dir))
			entries = append(entries, entry)
		}
		if !res.IsTruncated {
			break
		}
		startAfter = oss.StartAfter(res.StartAfter)
		continuationToken = oss.ContinuationToken(res.NextContinuationToken)
	}
	return entries, nil
}

type ReadDirFile struct {
	FS
	root              string
	startAfter        string
	continuationToken string
	isTruncated       bool
}

func NewReadDirFile(fs *FS, name string) *ReadDirFile {
	ret := new(ReadDirFile)
	ret.clt = fs.clt
	ret.root = name
	return ret
}

func (f *ReadDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry
	prefix := oss.Prefix(clearDirPath(f.root))
	res, err := f.clt.bucket.ListObjectsV2(prefix, oss.StartAfter(f.startAfter), oss.StartAfter(f.continuationToken), oss.Delimiter("/"), oss.MaxKeys(n))
	if err != nil {
		return entries, err
	}
	for _, obj := range res.Objects {
		entry := NewDirEntry(NewFileInfo(&obj))
		entries = append(entries, entry)
	}
	for _, dir := range res.CommonPrefixes {
		entry := NewDirEntry(NewFileInfoWithDir(dir))
		entries = append(entries, entry)
	}
	f.isTruncated = res.IsTruncated
	if !f.isTruncated {
		f.startAfter = res.StartAfter
		f.continuationToken = res.NextContinuationToken
	} else {
		f.startAfter = ""
		f.continuationToken = ""
	}
	return entries, nil
}

func (f *ReadDirFile) Reset() {
	f.isTruncated = false
	f.startAfter = ""
	f.continuationToken = ""
}

func (f *ReadDirFile) Completed() bool {
	return !f.isTruncated
}

type SubFS struct {
	FS
}

func (f *SubFS) Sub(name string) (fs.FS, error) {
	return NewReadDirFile(&f.FS, name), nil
}
