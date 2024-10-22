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

func Watch(cfg *config.Setting) (*watcher.Watcher, error) {
	op := fsnotify.Create | fsnotify.Write | fsnotify.Rename
	if cfg.Delete {
		op = op | fsnotify.Remove
	}
	w, err := watcher.NewWatcher(watcher.WithIgnoreHiddenFiles(cfg.IgnoreHiddenFiles), watcher.WithOpFilter(op))
	if err != nil {
		return nil, err
	}
	cm := new(lib.CommandManager)
	cm.Init()
	go func() {
		for {
			select {
			case event := <-w.Events:
				if err := doSync(cm, cfg, &event); err != nil {
					log.Println(err)
				}
			case err := <-w.Errors:
				log.Println(err)
			case <-w.Closed:
				return
			}
		}
	}()
	if err := w.AddRecursive(cfg.Local); err != nil {
		return nil, err
	}
	doSync(cm, cfg, nil)
	return w, nil
}

func doSync(cm *lib.CommandManager, setting *config.Setting, event *fsnotify.Event) error {
	args := []string{
		setting.Local,
		fmt.Sprintf("oss://%s", filepath.Join(setting.Bucket, setting.Prefix)),
	}
	var (
		TRUE      = true
		outputDir = filepath.Join(setting.Prefix, ".ossutil_output")
		cpDir     = filepath.Join(setting.Prefix, ".ossutil_checkoutpoint")
		backupDir = filepath.Join(setting.Prefix, ".ossutil_backup")
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
	options[lib.OptionDisableDirObject] = &TRUE
	options[lib.OptionDisableAllSymlink] = &TRUE
	options[lib.OptionForce] = &TRUE
	options[lib.OptionUpdate] = &TRUE
	if _, err := cm.RunCommand("sync", args, options); err != nil {
		return err
	}
	return nil
}
