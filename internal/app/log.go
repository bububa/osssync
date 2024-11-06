package app

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/atomic"

	"github.com/bububa/osssync/internal/service/log"
)

var logViewDisplayed = atomic.NewBool(false)

func LogWindow(a fyne.App) {
	if logViewDisplayed.Load() {
		return
	}
	logViewDisplayed.Store(true)
	windowTitle := lang.L("systembar.log")
	w := a.NewWindow(windowTitle)
	w.Resize(fyne.NewSize(600.0, 400))
	closed := make(chan struct{}, 1)
	tailer, err := log.TailStart()
	logsCache := make([]string, 0, 1000)
	logs := binding.NewStringList()
	stopped := atomic.NewBool(false)
	var (
		toggleBtn *widget.ToolbarAction
		toolbar   *widget.Toolbar
	)
	toggleBtn = widget.NewToolbarAction(theme.MediaStopIcon(), func() {
		if stopped.Load() {
			toggleBtn.SetIcon(theme.MediaStopIcon())
			tailer, _ = log.TailStart()
			stopped.Store(false)
		} else {
			toggleBtn.SetIcon(theme.MediaPlayIcon())
			log.TailStop(tailer)
			stopped.Store(true)
		}
	})
	toolbar = widget.NewToolbar(
		toggleBtn,
		widget.NewToolbarAction(theme.ContentClearIcon(), func() {
			logsCache = make([]string, 0, 1000)
			logs.Set(nil)
		}),
	)
	listView := widget.NewListWithData(logs,
		func() fyne.CanvasObject {
			v := widget.NewLabel("")
			// v.Wrapping = fyne.TextWrapBreak
			return v
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			v := o.(*widget.Label)
			v.Bind(i.(binding.String))
		},
	)
	out := container.NewBorder(toolbar, nil, nil, nil, toolbar, listView)
	w.SetContent(out)
	w.CenterOnScreen()
	w.Show()
	w.SetOnClosed(func() {
		close(closed)
		logViewDisplayed.Store(false)
	})
	if err != nil {
		dialog.ShowError(err, w)
	}
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		for {
			select {
			case line := <-tailer.Lines:
				if line != nil {
					logsCache = append([]string{line.Text}, logsCache...)
					logs.Prepend(line.Text)
				}
			case <-ticker.C:
				list, _ := logs.Get()
				list = append(logsCache, list...)
				logs.Set(list)
			case <-closed:
				log.TailCleanup(tailer)
				ticker.Stop()
				return
			}
		}
	}()
}
