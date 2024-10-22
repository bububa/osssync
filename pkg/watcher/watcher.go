// Package watcher implements recursive folder monitoring by wrapping the excellent fsnotify library
package watcher

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/fsnotify.v1"
)

// Watcher wraps fsnotify.Watcher. When fsnotify adds recursive watches, you should be able to switch your code to use fsnotify.Watcher
type Watcher struct {
	fsnotify *fsnotify.Watcher
	Events   chan fsnotify.Event
	Errors   chan error
	Closed   chan struct{}
	done     chan struct{}
	// filters filter hooks
	filters []FilterFileHookFunc
	op      fsnotify.Op
	// ignoreHidden ignore hidden files or not.
	ignoreHidden bool
	isClosed     bool
}

// NewWatcher establishes a new watcher with the underlying OS and begins waiting for events.
func NewWatcher(opts ...Option) (*Watcher, error) {
	fsWatch, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	m := &Watcher{}
	m.fsnotify = fsWatch
	m.Events = make(chan fsnotify.Event)
	m.Errors = make(chan error)
	m.Closed = make(chan struct{})
	m.done = make(chan struct{})
	for _, opt := range opts {
		opt(m)
	}

	go m.start()

	return m, nil
}

// Add starts watching the named file or directory (non-recursively).
func (m *Watcher) Add(name string) error {
	if m.isClosed {
		return errors.New("rfsnotify instance already closed")
	}
	return m.fsnotify.Add(name)
}

// AddRecursive starts watching the named directory and all sub-directories.
func (m *Watcher) AddRecursive(name string) error {
	if m.isClosed {
		return errors.New("rfsnotify instance already closed")
	}
	if err := m.watchRecursive(name, false); err != nil {
		return err
	}
	return nil
}

// Remove stops watching the the named file or directory (non-recursively).
func (m *Watcher) Remove(name string) error {
	return m.fsnotify.Remove(name)
}

// RemoveRecursive stops watching the named directory and all sub-directories.
func (m *Watcher) RemoveRecursive(name string) error {
	if err := m.watchRecursive(name, true); err != nil {
		return err
	}
	return nil
}

// Close removes all watches and closes the events channel.
func (m *Watcher) Close() error {
	if m.isClosed {
		return nil
	}
	close(m.done)
	m.isClosed = true
	return nil
}

func (m *Watcher) start() {
	for {
		select {

		case e := <-m.fsnotify.Events:
			s, err := os.Stat(e.Name)
			if err := m.filter(e.Name, s); err != nil {
				continue
			}
			if err == nil && s != nil && s.IsDir() {
				if e.Op&fsnotify.Create != 0 {
					m.watchRecursive(e.Name, false)
				}
			}
			// Can't stat a deleted directory, so just pretend that it's always a directory and
			// try to remove from the watch list...  we really have no clue if it's a directory or not...
			if e.Op&fsnotify.Remove != 0 {
				m.fsnotify.Remove(e.Name)
			}
			if m.op == 0 || e.Op&m.op != 0 {
				m.Events <- e
			}
		case e := <-m.fsnotify.Errors:
			m.Errors <- e

		case <-m.done:
			m.fsnotify.Close()
			close(m.Events)
			close(m.Errors)
			close(m.Closed)
			return
		}
	}
}

// watchRecursive adds all directories under the given one to the watch list.
// this is probably a very racey process. What if a file is added to a folder before we get the watch added?
func (m *Watcher) watchRecursive(path string, unWatch bool) error {
	err := filepath.Walk(path, func(walkPath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if err := m.filter(walkPath, fi); err != nil {
			return nil
		}
		if fi.IsDir() {
			if unWatch {
				if err = m.fsnotify.Remove(walkPath); err != nil {
					return err
				}
			} else {
				if err = m.fsnotify.Add(walkPath); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return err
}

func (w *Watcher) filter(path string, fi os.FileInfo) error {
	if w.ignoreHidden {
		if isHidden, err := isHiddenFile(path); err != nil {
			return err
		} else if isHidden {
			return ErrSkip
		}
	}

	for _, fn := range w.filters {
		if err := fn(fi, path); err != nil {
			return err
		}
	}
	return nil
}
