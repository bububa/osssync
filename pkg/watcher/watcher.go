// Package watcher implements recursive folder monitoring by wrapping the excellent fsnotify library
package watcher

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/atomic"

	"github.com/bububa/osssync/pkg"
	"github.com/bububa/osssync/pkg/fs/local"
)

type Event struct {
	SettingKey string
	File       *local.FileInfo
	Ori        *local.FileInfo
	HandlerKey string
	Op         fsnotify.Op
}

// Watcher wraps fsnotify.Watcher. When fsnotify adds recursive watches, you should be able to switch your code to use fsnotify.Watcher
type Watcher struct {
	fsnotify *fsnotify.Watcher
	isClosed *atomic.Bool
	cache    *pkg.Map[string, *local.FileInfo]
	Events   chan Event
	Errors   chan error
	Closed   chan struct{}
	done     chan struct{}
	root     string
	// filters filter hooks
	filters []FilterFileHookFunc
	op      fsnotify.Op
	// ignoreHidden ignore hidden files or not.
	ignoreHidden bool
}

// NewWatcher establishes a new watcher with the underlying OS and begins waiting for events.
func NewWatcher(opts ...Option) (*Watcher, error) {
	fsWatch, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	m := &Watcher{
		fsnotify: fsWatch,
		cache:    pkg.NewMap[string, *local.FileInfo](),
		Events:   make(chan Event),
		Errors:   make(chan error),
		Closed:   make(chan struct{}, 1),
		done:     make(chan struct{}, 1),
		isClosed: atomic.NewBool(false),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

func (m *Watcher) Start(name string) error {
	m.root = name
	if m.isClosed.Load() {
		return errors.New("rfsnotify instance already closed")
	}
	go m.start()
	return m.watchRecursive(name, false, true)
}

// Add starts watching the named file or directory (non-recursively).
func (m *Watcher) Add(name string) error {
	if m.isClosed.Load() {
		return errors.New("rfsnotify instance already closed")
	}
	return m.fsnotify.Add(name)
}

// AddRecursive starts watching the named directory and all sub-directories.
func (m *Watcher) AddRecursive(name string) error {
	if m.isClosed.Load() {
		return errors.New("rfsnotify instance already closed")
	}
	return m.watchRecursive(name, false, false)
}

// Remove stops watching the the named file or directory (non-recursively).
func (m *Watcher) Remove(name string) error {
	if m.isClosed.Load() {
		return errors.New("rfsnotify instance already closed")
	}
	return m.fsnotify.Remove(name)
}

// RemoveRecursive stops watching the named directory and all sub-directories.
func (m *Watcher) RemoveRecursive(name string) error {
	if m.isClosed.Load() {
		return errors.New("rfsnotify instance already closed")
	}
	return m.watchRecursive(name, true, false)
}

// Close removes all watches and closes the events channel.
func (m *Watcher) Close() error {
	if m.isClosed.Load() {
		return nil
	}
	close(m.done)
	m.isClosed.Store(true)
	return nil
}

func (m *Watcher) sync(cache map[string]*local.FileInfo, handlerKey string) {
	removes := make(map[string]*local.FileInfo, m.cache.Count())
	creates := make(map[string]*local.FileInfo, m.cache.Count())
	m.cache.Range(func(key string, fi *local.FileInfo) bool {
		if cache == nil {
			m.Events <- Event{
				Op:         fsnotify.Create,
				File:       fi,
				HandlerKey: handlerKey,
			}
			return true
		}
		oldFi, ok := cache[key]
		if !ok {
			creates[key] = fi
			return true
		}
		if fi.ModTime().After(oldFi.ModTime()) {
			m.Events <- Event{
				Op:         fsnotify.Write,
				File:       fi,
				HandlerKey: handlerKey,
			}
		}
		return true
	})
	for key, fi := range cache {
		if _, ok := m.cache.Load(key); !ok {
			removes[key] = fi
		}
	}

	// Check for renames and moves.
	for path1, info1 := range removes {
		for path2, info2 := range creates {
			if isSameFile(info1.FileInfo, info2.FileInfo) {
				e := Event{
					Op:         fsnotify.Rename,
					File:       info2,
					Ori:        info1,
					HandlerKey: handlerKey,
				}

				delete(removes, path1)
				delete(creates, path2)
				m.Events <- e
			}
		}
	}
	// Send all the remaining create and remove events.
	for _, info := range creates {
		m.Events <- Event{File: info, Op: fsnotify.Create, HandlerKey: handlerKey}
	}
	for _, info := range removes {
		m.Events <- Event{File: info, Op: fsnotify.Remove, HandlerKey: handlerKey}
	}
}

func (m *Watcher) start() {
	for {
		select {

		case e := <-m.fsnotify.Events:
			cache := m.cache.GetMap()
			if e.Op&fsnotify.Create == fsnotify.Create {
				if s, err := os.Stat(e.Name); err != nil {
					continue
				} else if s != nil && s.IsDir() {
					m.AddRecursive(e.Name)
				}
			}
			// Can't stat a deleted directory, so just pretend that it's always a directory and
			// try to remove from the watch list...  we really have no clue if it's a directory or not...
			if e.Op&fsnotify.Remove == fsnotify.Remove || e.Op&fsnotify.Rename == fsnotify.Rename {
				m.RemoveRecursive(e.Name)
			}
			m.watchRecursive(m.root, false, true)
			m.sync(cache, "")
		case e := <-m.fsnotify.Errors:
			if e != nil && !errors.Is(e, fsnotify.ErrClosed) {
				m.Errors <- e
			}
		case <-m.done:
			m.fsnotify.Close()
			close(m.Events)
			close(m.Errors)
			close(m.Closed)
			return
		}
	}
}

func (m *Watcher) Notify(handlerKey string) {
	m.sync(nil, handlerKey)
}

// watchRecursive adds all directories under the given one to the watch list.
// this is probably a very racey process. What if a file is added to a folder before we get the watch added?
func (m *Watcher) watchRecursive(path string, unWatch bool, updateCache bool) error {
	mp := make(map[string]os.FileInfo)
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
		} else if updateCache {
			mp[walkPath] = fi
		}
		return nil
	})
	if updateCache {
		m.cache.Range(func(walkPath string, oldFi *local.FileInfo) bool {
			if fi, ok := mp[walkPath]; !ok {
				m.cache.Delete(walkPath)
			} else if !fi.ModTime().After(oldFi.ModTime()) {
				delete(mp, walkPath)
			}
			return true
		})
		for walkPath, fi := range mp {
			m.cache.Store(walkPath, local.NewFileInfo(fi, local.WithPath(walkPath)))
			// if fp, err := os.Open(walkPath); err == nil {
			// 	defer fp.Close()
			// 	etag := GetEtagByReader(fp, fi.Size())
			// 	m.cache.Store(walkPath, &fsPkg.FileCache{Path: walkPath, File: fi, Etag: etag, ModTime: fi.ModTime()})
			// }
		}
	}
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
