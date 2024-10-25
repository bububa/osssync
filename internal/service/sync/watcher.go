package sync

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/aliyun/ossutil/lib"
	"gopkg.in/fsnotify.v1"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/pkg/watcher"
)

func Watch(cfg *config.Setting, msgCh chan *lib.CPMonitorSnap) (*watcher.Watcher, error) {
	op := fsnotify.Create | fsnotify.Write | fsnotify.Rename
	if cfg.Delete {
		op |= fsnotify.Remove
	}
	w, err := watcher.NewWatcher(watcher.WithIgnoreHiddenFiles(cfg.IgnoreHiddenFiles), watcher.WithOpFilter(op))
	if err != nil {
		return nil, err
	}
	syncCommand := newSyncer(cfg, msgCh)
	go func() {
		for {
			select {
			case event := <-w.Events:
				log.Println("fsnotify", event)
				if err := syncCommand.RunCommand(); err != nil {
					log.Println(err)
				}
			case err := <-w.Errors:
				log.Println("error", err)
			case <-w.Closed:
				return
			}
		}
	}()
	if err := w.AddRecursive(cfg.Local); err != nil {
		return nil, err
	}
	if err := syncCommand.RunCommand(); err != nil {
		log.Println(err)
	}
	return w, nil
}

func newSyncer(setting *config.Setting, monitor chan *lib.CPMonitorSnap) *lib.SyncCommand {
	args := []string{
		setting.Local,
		fmt.Sprintf("oss://%s", filepath.Join(setting.Bucket, setting.Prefix)),
	}
	var (
		TRUE      = true
		FALSE     = false
		outputDir = filepath.Join(setting.Local, ".ossutil_output")
		cpDir     = filepath.Join(setting.Local, ".ossutil_checkoutpoint")
		backupDir = filepath.Join(setting.Local, ".ossutil_backup")
	)
	_, options, _ := lib.ParseArgOptions()
	options[lib.OptionEndpoint] = &setting.Endpoint
	options[lib.OptionAccessKeyID] = &setting.AccessKeyID
	options[lib.OptionAccessKeySecret] = &setting.AccessKeySecret
	options[lib.OptionOutputDir] = &outputDir
	options[lib.OptionCheckpointDir] = &cpDir
	options[lib.OptionBackupDir] = &backupDir
	options[lib.OptionExclude] = "^.*"
	if setting.Delete {
		options[lib.OptionDelete] = &TRUE
	}
	if setting.IgnoreHiddenFiles {
		options[lib.OptionIgnoreHiddenFile] = &TRUE
	} else {
		options[lib.OptionIgnoreHiddenFile] = &FALSE
	}
	options[lib.OptionDisableDirObject] = &TRUE
	options[lib.OptionDisableAllSymlink] = &TRUE
	options[lib.OptionForce] = &TRUE
	options[lib.OptionUpdate] = &TRUE
	syncCommand := lib.NewSyncCommand()
	syncCommand.Init(args, options)
	syncCommand.SetMonitor(monitor)
	return syncCommand
}
