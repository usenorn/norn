package mailtemplate

import (
	"embed"
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	"net/url"
	"strings"
	texttemplate "text/template"
)

//go:embed layout.html
var layout embed.FS

type Shell struct {
	Subject   string
	Preheader string
	Eyebrow   string
	LogoURL   string
	Content   any
}

var functions = htmltemplate.FuncMap{"button": button, "row": row}

func LogoURL(baseURL string) string {
	if baseURL == "" {
		return ""
	}

	asset, err := url.JoinPath(baseURL, "/email/norn-mark.png")
	if err != nil {
		return ""
	}

	return asset
}

func HTML(content fs.FS, name string) (*htmltemplate.Template, error) {
	shell, err := htmltemplate.New("layout.html").Funcs(functions).ParseFS(layout, "layout.html")
	if err != nil {
		return nil, fmt.Errorf("parse mail layout: %w", err)
	}

	parsed, err := shell.ParseFS(content, name)
	if err != nil {
		return nil, fmt.Errorf("parse mail content %s: %w", name, err)
	}

	return parsed, nil
}

func MustHTML(content fs.FS, name string) *htmltemplate.Template {
	parsed, err := HTML(content, name)
	if err != nil {
		panic(err)
	}

	return parsed
}

func Render(parsed *htmltemplate.Template, shell Shell) (string, error) {
	var out strings.Builder
	if err := parsed.ExecuteTemplate(&out, "layout.html", shell); err != nil {
		return "", fmt.Errorf("render mail html: %w", err)
	}

	return out.String(), nil
}

func RenderPlain(parsed *texttemplate.Template, name string, content any) (string, error) {
	var out strings.Builder
	if err := parsed.ExecuteTemplate(&out, name, content); err != nil {
		return "", fmt.Errorf("render mail text: %w", err)
	}

	return strings.TrimSpace(out.String()) + "\n", nil
}

func LogoURLFrom(instanceURL string) string {
	parsed, err := url.Parse(instanceURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return parsed.Scheme + "://" + parsed.Host + "/email/norn-mark.png"
}

func button(label, href string) htmltemplate.HTML {
	safeLabel := htmltemplate.HTMLEscapeString(label)
	safeHref := htmltemplate.HTMLEscapeString(href)

	markup := `<table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin:24px 0 22px;">
  <tr><td class="norn-btn" align="center" bgcolor="#0f1116" style="background:#0f1116;border-radius:5px;">
    <!--[if mso]><v:roundrect xmlns:v="urn:schemas-microsoft-com:vml" xmlns:w="urn:schemas-microsoft-com:office:word" href="` + safeHref + `" style="height:42px;v-text-anchor:middle;width:250px;" arcsize="12%" stroke="f" fillcolor="#0f1116"><w:anchorlock/><center style="color:#ffffff;font-family:Arial,sans-serif;font-size:15px;font-weight:600;"><![endif]-->
    <a href="` + safeHref + `" style="display:inline-block;padding:12px 26px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:15px;font-weight:600;line-height:18px;color:#ffffff;text-decoration:none;border-radius:5px;">` + safeLabel + `</a>
    <!--[if mso]></center></v:roundrect><![endif]-->
  </td></tr>
</table>`

	return htmltemplate.HTML(markup)
}

func row(label string, value any) htmltemplate.HTML {
	safeLabel := htmltemplate.HTMLEscapeString(label)
	safeValue := htmltemplate.HTMLEscapeString(fmt.Sprint(value))

	markup := `<table role="presentation" class="norn-meta-row" width="100%" cellpadding="0" cellspacing="0" border="0" style="width:100%;">
  <tr>
    <td class="norn-meta-label" width="34%" style="padding:9px 12px 9px 0;border-top:1px solid #e6e8ec;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:11px;letter-spacing:0.08em;text-transform:uppercase;color:#8a93a6;vertical-align:top;">` + safeLabel + `</td>
    <td class="norn-meta-value" style="padding:9px 0;border-top:1px solid #e6e8ec;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:14px;line-height:1.5;color:#2b303a;vertical-align:top;">` + safeValue + `</td>
  </tr>
</table>`

	return htmltemplate.HTML(markup)
}
