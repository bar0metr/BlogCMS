package web

import (
	"fmt"
	"html/template"
	"io"
	"path/filepath"

	"blogcms/internal/web/templates"
)

type Renderer interface {
	Render(w io.Writer, name string, data any) error
}

type TemplateRenderer struct {
	templates map[string]*template.Template
}

func NewTemplateRenderer() (*TemplateRenderer, error) {
	funcs := template.FuncMap{
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}

	layoutBytes, err := templates.FS.ReadFile("layout.html")
	if err != nil {
		return nil, fmt.Errorf("read layout: %w", err)
	}

	entries, err := templates.FS.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read templates dir: %w", err)
	}

	out := make(map[string]*template.Template)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "layout.html" {
			continue
		}
		b, err := templates.FS.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", name, err)
		}

		t := template.New("root").Funcs(funcs)
		if _, err := t.Parse(string(layoutBytes)); err != nil {
			return nil, fmt.Errorf("parse layout: %w", err)
		}
		if _, err := t.Parse(string(b)); err != nil {
			return nil, fmt.Errorf("parse template %s: %w", name, err)
		}
		out[filepath.Base(name)] = t
	}

	return &TemplateRenderer{templates: out}, nil
}

func (r *TemplateRenderer) Render(w io.Writer, name string, data any) error {
	t, ok := r.templates[name]
	if !ok {
		return fmt.Errorf("template not found: %s", name)
	}
	return t.ExecuteTemplate(w, "layout", data)
}
