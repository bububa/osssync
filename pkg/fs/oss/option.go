package oss

type Option func(fs *FS)

func WithPrefix(prefix string) Option {
	return func(fs *FS) {
		fs.prefix = prefix
	}
}

func WithLocal(local string) Option {
	return func(fs *FS) {
		fs.local = local
	}
}
