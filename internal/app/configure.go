package app

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service"
)

func Configure(a fyne.App) {
	configData, _ := service.LoadConfigString()
	w := a.NewWindow(fmt.Sprintf("%s Configure", config.AppName))
	w.Resize(fyne.NewSize(400.0, 0))
	textArea := widget.NewMultiLineEntry()
	textArea.SetText(string(configData))
	textArea.SetMinRowsVisible(10)
	cancelBtn := widget.NewButton("Cancel", func() {
		w.Close()
	})
	cancelBtn.Importance = widget.LowImportance
	confirmBtn := widget.NewButtonWithIcon("Submit", theme.DocumentSaveIcon(), func() {
		content := textArea.Text
		service.WriteConfigFile([]byte(strings.TrimSpace(content)))
		w.Close()
	})
	confirmBtn.Importance = widget.HighImportance
	btnContainer := container.NewHBox(layout.NewSpacer(), cancelBtn, confirmBtn)
	inner := container.NewVBox(textArea, btnContainer)
	out := container.NewPadded(inner)
	w.SetContent(out)
	w.CenterOnScreen()
	w.Show()
}
