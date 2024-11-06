package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/bububa/osssync/internal/config"
	"github.com/bububa/osssync/internal/service"
	"github.com/bububa/osssync/pkg"
)

var configWindowOpened = pkg.NewMap[string, struct{}]()

func updateConfig(a fyne.App, setting config.Setting) error {
	var exists bool
	cfg := *service.Config()
	for idx, s := range cfg.Settings {
		if s.Key() == setting.Key() {
			cfg.Settings[idx] = setting
			exists = true
			break
		}
	}
	if !exists {
		return errors.New(lang.L("error.settingNotExist"))
	}
	return service.SaveConfig(&cfg)
}

func createConfig(a fyne.App, setting config.Setting) error {
	cfg := *service.Config()
	for _, s := range cfg.Settings {
		if s.Key() == setting.Key() {
			return errors.New(lang.L("error.duplicateSetting"))
		}
	}
	cfg.Settings = append(cfg.Settings, setting)
	return service.SaveConfig(&cfg)
}

func deleteConfig(setting config.Setting) error {
	cfg := *service.Config()
	settings := make([]config.Setting, 0, len(cfg.Settings))
	for _, s := range cfg.Settings {
		if s.Key() == setting.Key() {
			continue
		}
		settings = append(settings, s)
	}
	cfg.Settings = settings
	return service.SaveConfig(&cfg)
}

type editCallbackFunc func(a fyne.App, setting config.Setting) error

func EditSetting(a fyne.App, cfg config.Setting, isNew bool) {
	windowTitle := lang.L("config.create")
	key := cfg.Key()
	var (
		callback editCallbackFunc
		isUpdate bool
	)
	if isNew || cfg == config.EmptySetting {
		callback = createConfig
	} else {
		callback = updateConfig
		isUpdate = true
	}
	if cfg != config.EmptySetting && cfg.Name != "" {
		if _, ok := configWindowOpened.Load(key); ok {
			return
		}
		windowTitle = fmt.Sprintf("%s %s", cfg.DisplayName(), lang.L("config.setting"))
		configWindowOpened.Store(key, struct{}{})
	}
	w := a.NewWindow(windowTitle)
	w.Resize(fyne.NewSize(600.0, 0))
	namePointer := &cfg.Name
	nameData := binding.BindString(namePointer)
	nameField := widget.NewEntryWithData(nameData)
	nameField.Validator = func(str string) error {
		if str == "" {
			return fmt.Errorf("%s%s", lang.L("config.name"), lang.L("isRequired"))
		}
		return nil
	}
	localPointer := &cfg.Local
	localData := binding.BindString(localPointer)
	folderDialog := dialog.NewFolderOpen(func(reader fyne.ListableURI, err error) {
		if reader != nil {
			*localPointer = reader.Path()
			localData.Reload()
		}
	}, w)
	folderDialog.SetConfirmText(lang.L("chooseConfirm"))
	if cfg.Local != "" {
		if uri, err := storage.ParseURI(fmt.Sprintf("file://%s", filepath.ToSlash(cfg.Local))); err == nil {
			if uri, err := storage.ListerForURI(uri); err == nil {
				folderDialog.SetLocation(uri)
			}
		}
	}
	folderBtn := widget.NewButton(lang.L("config.chooseFolder"), folderDialog.Show)
	localField := widget.NewEntryWithData(localData)
	localField.Validator = func(str string) error {
		_, err := os.Stat(str)
		return err
	}
	// localField.ActionItem = folderBtn
	localContainer := container.NewStack(container.NewBorder(nil, nil, nil, folderBtn, localField, folderBtn))
	endpointPointer := &cfg.Endpoint
	endpointData := binding.BindString(endpointPointer)
	endpointField := widget.NewEntryWithData(endpointData)
	endpointField.Validator = func(str string) error {
		if str == "" {
			return fmt.Errorf("%s%s", lang.L("config.endpoint"), lang.L("isRequired"))
		}
		return nil
	}
	accessKeyIDPointer := &cfg.AccessKeyID
	accessKeyIDData := binding.BindString(accessKeyIDPointer)
	accessKeyIDField := widget.NewEntryWithData(accessKeyIDData)
	accessKeyIDField.Validator = func(str string) error {
		if str == "" {
			return fmt.Errorf("%s%s", lang.L("config.accessKeyID"), lang.L("isRequired"))
		}
		return nil
	}
	accessKeySecretPointer := &cfg.AccessKeySecret
	accessKeySecretData := binding.BindString(accessKeySecretPointer)
	accessKeySecretField := widget.NewEntryWithData(accessKeySecretData)
	accessKeySecretField.Validator = func(str string) error {
		if str == "" {
			return fmt.Errorf("%s%s", lang.L("config.accessKeySecret"), lang.L("isRequired"))
		}
		return nil
	}
	bucketPointer := &cfg.Bucket
	bucketData := binding.BindString(bucketPointer)
	bucketField := widget.NewEntryWithData(bucketData)
	bucketField.Validator = func(str string) error {
		if str == "" {
			return fmt.Errorf("%s%s", lang.L("config.bucket"), lang.L("isRequired"))
		}
		return nil
	}
	prefixPointer := &cfg.Prefix
	prefixData := binding.BindString(prefixPointer)
	prefixField := widget.NewEntryWithData(prefixData)
	prefixField.Validator = func(str string) error {
		if str == "" {
			return fmt.Errorf("%s%s", lang.L("config.prefix"), lang.L("isRequired"))
		}
		return nil
	}
	localField.OnChanged = func(str string) {
		if cfg.Prefix == "" && cfg.Local != "" {
			cfg.Prefix = filepath.Base(cfg.Local)
		}
		prefixData.Reload()
	}
	ignoreHiddenPointer := &cfg.IgnoreHiddenFiles
	ignoreHiddenData := binding.BindBool(ignoreHiddenPointer)
	ignoreHiddenField := widget.NewCheckWithData("", ignoreHiddenData)
	deletePointer := &cfg.Delete
	deleteData := binding.BindBool(deletePointer)
	deleteField := widget.NewCheckWithData("", deleteData)
	if isUpdate {
		folderBtn.Disable()
		localField.Disable()
		bucketField.Disable()
		prefixField.Disable()
	}
	form := &widget.Form{
		Items: []*widget.FormItem{ // we can specify items in the constructor
			{Text: lang.L("config.name"), Widget: nameField},
			{Text: lang.L("config.local"), Widget: localContainer},
			{Text: lang.L("config.endpoint"), Widget: endpointField},
			{Text: lang.L("config.accessKeyID"), Widget: accessKeyIDField},
			{Text: lang.L("config.accessKeySecret"), Widget: accessKeySecretField},
			{Text: lang.L("config.bucket"), Widget: bucketField},
			{Text: lang.L("config.prefix"), Widget: prefixField},
			{Text: lang.L("config.ignoreHiddenFiles"), Widget: ignoreHiddenField},
			{Text: lang.L("config.delete"), Widget: deleteField},
		},
		SubmitText: lang.L("Save"),
		OnSubmit: func() { // optional, handle form submission
			if err := callback(a, cfg); err != nil {
				dialog.ShowError(err, w)
				return
			}
			w.Close()
		},
	}
	form.Validate()
	// configData, _ := service.LoadConfigString()
	// textArea := widget.NewMultiLineEntry()
	// textArea.SetText(string(configData))
	// textArea.SetMinRowsVisible(10)
	// cancelBtn := widget.NewButton("Cancel", func() {
	// 	w.Close()
	// })
	// cancelBtn.Importance = widget.LowImportance
	// confirmBtn := widget.NewButtonWithIcon("Submit", theme.DocumentSaveIcon(), func() {
	// 	content := textArea.Text
	// 	service.WriteConfigFile([]byte(strings.TrimSpace(content)))
	// 	w.Close()
	// })
	// confirmBtn.Importance = widget.HighImportance
	// btnContainer := container.NewHBox(layout.NewSpacer(), cancelBtn, confirmBtn)
	// inner := container.NewVBox(textArea, btnContainer)
	// out := container.NewPadded(form)
	w.SetContent(form)
	w.SetOnClosed(func() {
		configWindowOpened.Delete(key)
	})
	w.CenterOnScreen()
	w.Show()
}
