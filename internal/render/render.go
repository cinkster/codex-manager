package render

import (
	"embed"
	"html/template"
	"io"
	"time"
)

//go:embed templates/*.html
var templatesFS embed.FS

// Renderer loads and executes HTML templates.
type Renderer struct {
	templates *template.Template
}

// New loads embedded templates.
func New() (*Renderer, error) {
	funcs := template.FuncMap{
		"formatTime": formatTime,
	}
	tmpl, err := template.New("root").Funcs(funcs).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{templates: tmpl}, nil
}

// Execute renders a named template.
func (r *Renderer) Execute(w io.Writer, name string, data any) error {
	return r.templates.ExecuteTemplate(w, name, data)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
