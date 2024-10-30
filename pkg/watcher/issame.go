//go:build !windows
// +build !windows

package watcher

import "os"

func isSameFile(fi1, fi2 os.FileInfo) bool {
	return os.SameFile(fi1, fi2)
}
