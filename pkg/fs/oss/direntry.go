package oss

import "io/fs"

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
