package watcher

import (
	"errors"
	"os"
	"regexp"

	"gopkg.in/fsnotify.v1"
)

// ErrSkip is less of an error, but more of a way for path hooks to skip a file or
// directory.
var ErrSkip = errors.New("error: skipping file")

type Option = func(w *Watcher)

// FilterFileHookFunc is a function that is called to filter files during listings.
// If a file is ok to be listed, nil is returned otherwise ErrSkip is returned.
type FilterFileHookFunc func(info os.FileInfo, fullPath string) error

// RegexFilterHook is a function that accepts or rejects a file
// for listing based on whether it's filename or full path matches
// a regular expression.
func RegexFilterHook(r *regexp.Regexp, useFullPath bool) FilterFileHookFunc {
	return func(info os.FileInfo, fullPath string) error {
		str := info.Name()

		if useFullPath {
			str = fullPath
		}

		// Match
		if r.MatchString(str) {
			return nil
		}

		// No match.
		return ErrSkip
	}
}

// WithFilterHook
func WithFilterHook(f FilterFileHookFunc) Option {
	return func(w *Watcher) {
		w.filters = append(w.filters, f)
	}
}

// WithIgnoreHiddenFiles sets the watcher to ignore any file or directory
// that starts with a dot.
func WithIgnoreHiddenFiles(ignore bool) Option {
	return func(w *Watcher) {
		w.ignoreHidden = ignore
	}
}

// WithEventFilter filters which event op types should be returned
// when an event occurs.
func WithOpFilter(op fsnotify.Op) Option {
	return func(w *Watcher) {
		w.op = op
	}
}
