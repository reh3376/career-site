package email

import (
	"bytes"
	"embed"
	"fmt"
	htmltmpl "html/template"
	texttmpl "text/template"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

// Template pairs a text/plain template with an optional HTML template of
// the same base name. Every transactional email in the codebase goes
// through Render() so the sending call site never touches template syntax.
type Template struct {
	Name string
	Text *texttmpl.Template
	HTML *htmltmpl.Template
}

func mustLoad(name string) *Template {
	t := &Template{Name: name}
	textBytes, err := templatesFS.ReadFile("templates/" + name + ".txt.tmpl")
	if err != nil {
		panic(fmt.Errorf("load %s.txt.tmpl: %w", name, err))
	}
	t.Text = texttmpl.Must(texttmpl.New(name + ".txt").Parse(string(textBytes)))

	htmlBytes, err := templatesFS.ReadFile("templates/" + name + ".html.tmpl")
	if err == nil {
		t.HTML = htmltmpl.Must(htmltmpl.New(name + ".html").Parse(string(htmlBytes)))
	}
	return t
}

// Registered templates. Add a var + mustLoad line, drop the .tmpl files
// in templates/, and it's usable from handlers.
var (
	VerifyTemplate          = mustLoad("verify")
	ApprovalRequestTemplate = mustLoad("approval_request")
)

// Render returns the rendered text and (optional) HTML bodies for the
// given data. If a template has no HTML variant, htmlOut is empty.
func (t *Template) Render(data any) (textOut, htmlOut string, err error) {
	var tb, hb bytes.Buffer
	if err := t.Text.Execute(&tb, data); err != nil {
		return "", "", fmt.Errorf("render text: %w", err)
	}
	if t.HTML != nil {
		if err := t.HTML.Execute(&hb, data); err != nil {
			return "", "", fmt.Errorf("render html: %w", err)
		}
	}
	return tb.String(), hb.String(), nil
}
