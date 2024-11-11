package oss

import (
	"io/fs"
	"time"
)

// DirEntry implements fs.DirEntry.
type DirEntry struct {
	info *FileInfo
}

func NewDirEntry(info *FileInfo) *DirEntry {
	return &DirEntry{
		info: info,
	}
}

func (d DirEntry) Path() string {
	return d.info.Path()
}

func (d DirEntry) Dir() string {
	return d.info.Dir()
}

func (d DirEntry) Name() string {
	return d.info.Name()
}

func (d DirEntry) IsDir() bool {
	return d.info.IsDir()
}

func (d DirEntry) Type() fs.FileMode {
	return d.info.Mode()
}

func (d DirEntry) Info() (fs.FileInfo, error) {
	if d.info == nil {
		return nil, fs.ErrNotExist
	}
	return d.info, nil
}

func (d *DirEntry) SetPath(path string) {
	if d.info == nil {
		return
	}
	d.info.SetPath(path)
}

func (d *DirEntry) SetInfo(info *FileInfo) {
	d.info = info
}

func (d *DirEntry) UpdateModTime(t time.Time) {
	d.info.UpdateModTime(t)
}
