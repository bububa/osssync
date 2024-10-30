package app

import (
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
		for {
			select {
			case line := <-tailer.Lines:
				if line != nil {
					logs.Prepend(line.Text)
				}
			case <-closed:
				log.TailCleanup(tailer)
				return
			}
		}
	}()
}
