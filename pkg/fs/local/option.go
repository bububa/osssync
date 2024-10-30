package local

type Option func(*FileInfo)

func WithPath(path string) Option {
	return func(fi *FileInfo) {
		fi.path = path
	}
}

func WithEtag(etag string) Option {
	return func(fi *FileInfo) {
		fi.etag = etag
	}
}
