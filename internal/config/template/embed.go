package template

import (
	"embed"
	"log"
	"text/template"
)

//go:embed "config.tpl"
var templateFS embed.FS

var tpl *template.Template

func init() {
	if t, err := template.New("").ParseFS(templateFS, "config.tpl"); err != nil {
		log.Fatalln(err)
	} else {
		tpl = t
	}
}

func Template() *template.Template {
	return tpl
}
