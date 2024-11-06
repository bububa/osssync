package oss

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
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
	return filepath.Clean(filepath.ToSlash(dir)) + "/"
}

func cleanRemotePath(name string) string {
	return filepath.Clean(filepath.ToSlash(name))
}

func cleanLocalPath(name string) string {
	if ret, err := filepath.Abs(filepath.ToSlash(name)); err == nil {
		return ret
	}
	return cleanRemotePath(name)
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

func (f *FS) Open(ctx context.Context, name string) (fs.File, error) {
	name = f.PathAddPrefix(name)
	header, err := f.clt.bucket.GetObjectDetailedMeta(name, oss.WithContext(ctx))
	if err != nil {
		if e, ok := err.(oss.ServiceError); ok && e.Code == "NoSuchKey" {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	info := NewFileInfoWithHeader(f.PathRemovePrefix(name), header)
	return NewFile(f.clt.bucket, info), nil
}

func (f *FS) ReadFile(ctx context.Context, name string) ([]byte, error) {
	name = f.PathAddPrefix(name)
	body, err := f.clt.bucket.GetObject(name, oss.WithContext(ctx))
	if err != nil {
		if e, ok := err.(oss.ServiceError); ok && e.Code == "NoSuchKey" {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	defer body.Close()
	return io.ReadAll(body)
}

func (f *FS) Stat(ctx context.Context, name string) (fs.FileInfo, error) {
	file, err := f.Open(ctx, name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	return file.Stat()
}

func (f *FS) Exists(ctx context.Context, key string, etag string) bool {
	key = f.PathAddPrefix(key)
	options := []oss.Option{oss.WithContext(ctx)}
	if etag != "" {
		options = append(options, oss.IfNoneMatch(etag))
	}
	ret, _ := f.clt.bucket.IsObjectExist(key, options...)
	return ret
}

func (f *FS) Upload(ctx context.Context, key string, r io.Reader) error {
	key = f.PathAddPrefix(key)
	opts := []oss.Option{
		oss.WithContext(ctx),
		oss.ACL(oss.ACLPrivate),
	}
	return f.clt.bucket.PutObject(key, r, opts...)
}

func (f *FS) UploadFile(ctx context.Context, localFile *local.FileInfo) error {
	remotePath, err := f.RemotePathFromLocalFile(localFile)
	if err != nil {
		return err
	}

	if s, err := f.Stat(ctx, remotePath); err == nil {
		if s.ModTime().After(localFile.ModTime()) {
			return nil
		}
	}
	opts := []oss.Option{
		oss.WithContext(ctx),
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

func (f *FS) Remove(ctx context.Context, keys ...string) ([]string, error) {
	fkeys := make([]string, 0, len(keys))
	for _, v := range keys {
		fkeys = append(fkeys, f.PathAddPrefix(v))
	}
	res, err := f.clt.bucket.DeleteObjects(fkeys, oss.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	mp := make(map[string]struct{}, len(res.DeletedObjects))
	for _, v := range res.DeletedObjects {
		f.listener.RemoveListener(v).ProgressChanged(&oss.ProgressEvent{
			EventType: oss.TransferCompletedEvent,
		})
		mp[v] = struct{}{}
	}
	for _, v := range fkeys {
		if _, ok := mp[v]; !ok {
			f.listener.RemoveListener(v).ProgressChanged(&oss.ProgressEvent{
				EventType: oss.TransferFailedEvent,
			})
		}
	}
	return res.DeletedObjects, nil
}

func (f *FS) RemoveAll(ctx context.Context, dir string) ([]string, error) {
	dir = f.PathAddPrefix(dir)
	var ret []string
	_, err := f.clt.list(ctx, dir, func(list []oss.ObjectProperties) error {
		keys := make([]string, 0, len(list))
		for _, v := range list {
			keys = append(keys, v.Key)
		}
		if list, err := f.Remove(ctx, keys...); err != nil {
			return err
		} else {
			ret = append(ret, list...)
		}
		return nil
	})
	return ret, err
}

func (f *FS) Rename(ctx context.Context, src string, dist string) error {
	src = f.PathAddPrefix(src)
	dist = f.PathAddPrefix(dist)
	if err := f.Copy(ctx, src, dist); err != nil {
		return err
	}
	if _, err := f.Remove(ctx, src); err != nil {
		return err
	}
	return nil
}

func (f *FS) RenameDir(ctx context.Context, src string, dist string) error {
	src = f.PathAddPrefix(src)
	dist = f.PathAddPrefix(dist)
	if err := f.Copy(ctx, src, dist); err != nil {
		return err
	}
	_, err := f.clt.list(ctx, src, func(list []oss.ObjectProperties) error {
		keys := make([]string, 0, len(list))
		for _, v := range list {
			keys = append(keys, v.Key)
			target := cleanRemotePath(filepath.Join(dist, filepath.Base(v.Key)))
			if err := f.Copy(ctx, v.Key, target); err != nil {
				return err
			}
		}
		if _, err := f.Remove(ctx, keys...); err != nil {
			return err
		} else {
		}
		return nil
	})
	return err
}

func (f *FS) Copy(ctx context.Context, src string, dist string) error {
	src = f.PathAddPrefix(src)
	dist = f.PathAddPrefix(dist)
	_, err := f.clt.bucket.CopyObject(src, dist, oss.Progress(f.listener.CopyListener(src, dist)), oss.WithContext(ctx))
	return err
}

func (f *FS) Download(ctx context.Context, remotePath string, localPath string) error {
	remotePath = f.PathAddPrefix(remotePath)
	fn := filepath.Base(remotePath)
	localFile := filepath.Join(localPath, fn)
	cpDir := f.downloadCheckoutPointPath(localPath, fn)
	return f.clt.bucket.DownloadFile(remotePath, localFile, DefaultPartSize, oss.Checkpoint(true, cpDir), oss.Progress(f.listener.DownloadListener(remotePath, localFile)), oss.WithContext(ctx))
}

func (f *FS) RemotePathFromLocalFile(localFile *local.FileInfo) (string, error) {
	name := cleanLocalPath(localFile.Path())
	if !strings.HasPrefix(name, f.local) {
		return "", errors.New("invalid local file, file not inside local setting path")
	}
	return strings.Replace(name, f.local, f.prefix, 1), nil
}

func (f *FS) PathRemovePrefix(name string) string {
	name = cleanRemotePath(name)
	if !strings.HasPrefix(name, f.prefix) {
		return name
	}
	ret, _ := filepath.Rel(f.prefix, name)
	return ret
}

func (f *FS) PathAddPrefix(name string) string {
	name = cleanRemotePath(name)
	if strings.HasPrefix(name, f.prefix) {
		return name
	}
	return filepath.Join(f.prefix, name)
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

func (f *FS) Root() string {
	return f.prefix
}

func (f *FS) RootEntry() *DirEntry {
	return NewDirEntry(NewFileInfoWithDir(f.Root()))
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

func (fs *FS) ReadDirFile(name string) *ReadDirFile {
	return NewReadDirFile(fs, name)
}

type ReadDirFS struct {
	FS
}

func NewReadDirFS(fs *FS) *ReadDirFS {
	ret := new(ReadDirFS)
	ret.clt = fs.clt
	ret.prefix = fs.prefix
	return ret
}

func (f *ReadDirFS) ReadDir(ctx context.Context, name string) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry
	name = f.PathAddPrefix(name)
	prefix := oss.Prefix(clearDirPath(name))
	listType := oss.ListType(2)
	continuationToken := oss.ContinuationToken("")
	startAfter := oss.StartAfter("")
	for {
		res, err := f.clt.bucket.ListObjectsV2(prefix, listType, startAfter, continuationToken, oss.Delimiter("/"), oss.MaxKeys(MaxKeys), oss.WithContext(ctx))
		if err != nil {
			return entries, err
		}
		for _, obj := range res.Objects {
			obj.Key = f.PathRemovePrefix(obj.Key)
			entry := NewDirEntry(NewFileInfo(&obj))
			entries = append(entries, entry)
		}
		for _, dir := range res.CommonPrefixes {
			dir = f.PathRemovePrefix(dir)
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
	started           bool
	isTruncated       bool
}

func NewReadDirFile(fs *FS, name string) *ReadDirFile {
	ret := new(ReadDirFile)
	ret.clt = fs.clt
	ret.prefix = fs.prefix
	ret.root = clearDirPath(name)
	return ret
}

func (f *ReadDirFile) ReadDir(ctx context.Context, n int) ([]fs.DirEntry, error) {
	f.started = true
	var entries []fs.DirEntry
	prefix := oss.Prefix(clearDirPath(f.PathAddPrefix(f.Dir())))
	listType := oss.ListType(2)
	res, err := f.clt.bucket.ListObjectsV2(prefix, listType, oss.StartAfter(f.startAfter), oss.ContinuationToken(f.continuationToken), oss.Delimiter("/"), oss.MaxKeys(n), oss.WithContext(ctx))
	if err != nil {
		return entries, err
	}
	for _, obj := range res.Objects {
		obj.Key = f.PathRemovePrefix(obj.Key)
		entry := NewDirEntry(NewFileInfo(&obj))
		entries = append(entries, entry)
	}
	for _, dir := range res.CommonPrefixes {
		dir = f.PathRemovePrefix(dir)
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

func (f *ReadDirFile) Dir() string {
	return f.root
}

func (f *ReadDirFile) DirEntry() *DirEntry {
	return NewDirEntry(NewFileInfoWithDir(f.Dir()))
}

func (f *ReadDirFile) Reset() {
	f.isTruncated = false
	f.startAfter = ""
	f.continuationToken = ""
	f.started = false
}

func (f *ReadDirFile) Completed() bool {
	return !f.isTruncated && f.started
}
