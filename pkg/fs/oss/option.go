package oss

import (
	"strings"
)

type Option func(fs *FS)

func WithPrefix(prefix string) Option {
	return func(fs *FS) {
		fs.prefix = strings.TrimPrefix(clearDirPath(prefix), "/")
	}
}

func WithLocal(local string) Option {
	return func(fs *FS) {
		fs.local = clearDirPath(local)
	}
}

func WithIgnoreHidden(ignore bool) Option {
	return func(fs *FS) {
		fs.ignoreHidden = ignore
	}
}
