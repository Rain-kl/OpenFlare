package automation

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/gin-gonic/gin"
)

var browserPage = template.Must(template.New("automation").Parse(`<!doctype html>
<html lang="{{.Language}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} · OpenFlare</title>
  <style>
    :root { color-scheme: light dark }
    main { max-width: 44rem; margin: 4rem auto; padding: 0 1.25rem; font-family: system-ui, sans-serif; line-height: 1.55 }
    h1 { font-size: 1.65rem }
    textarea { width: 100%; box-sizing: border-box; padding: 1rem; font: inherit }
    button { font: inherit; padding: .4rem .85rem; margin: .35rem .5rem .35rem 0 }
    form { margin-top: 1rem }
  </style>
</head>
<body><main>
  <h1>{{.Title}}</h1>
  <p>{{.Detail}}</p>
  {{if .Intent}}
    <dl><dt>{{.Labels.Client}}</dt><dd>{{.Client}}</dd><dt>{{.Labels.Callback}}</dt><dd><code>{{.Redirect}}</code></dd><dt>{{.Labels.Scopes}}</dt><dd><code>{{.Scopes}}</code></dd></dl>
    <form method="post" action="{{.Action}}">
      <input type="hidden" name="intent" value="{{.Intent}}">
      {{if .AllowRead}}<button name="decision" value="read">{{.Labels.Read}}</button>{{end}}
      {{if .AllowWrite}}<button name="decision" value="operate">{{.Labels.Operate}}</button>{{end}}
      <button name="decision" value="deny">{{.Labels.Deny}}</button>
    </form>
  {{else}}
    <textarea id="setup" rows="9" readonly aria-label="{{.Labels.Copy}}">{{.Setup}}</textarea>
    <button id="copy" type="button">{{.Labels.Copy}}</button>
    <span id="status" role="status" data-copied="{{.Labels.Copied}}"></span>
    <script nonce="{{.Nonce}}">
      document.getElementById('copy').addEventListener('click', async () => {
        const field = document.getElementById('setup');
        field.select();
        try {
          await navigator.clipboard.writeText(field.value);
          const status = document.getElementById('status');
          status.textContent = status.dataset.copied;
        } catch {
          field.focus();
        }
      });
    </script>
    <form method="post" action="{{.Action}}">
      <button>{{.Labels.Revoke}}</button>
    </form>
  {{end}}
</main></body></html>`))

type pageLabels struct {
	Consent, Connect, ConsentDetail, ConnectDetail string
	Client, Callback, Scopes, Read, Operate, Deny  string
	Copy, Copied, Revoke, Setup                    string
}

//go:embed pages.json
var pageTranslationsJSON []byte

var pageTranslations = loadPageTranslations()

func loadPageTranslations() map[string]pageLabels {
	var translations map[string]pageLabels
	if err := json.Unmarshal(pageTranslationsJSON, &translations); err != nil {
		panic(err)
	}
	return translations
}

type pageData struct {
	Language, Title, Detail, Action, Intent, Client, Scopes, Setup, Nonce string
	Redirect                                                              string
	AllowRead, AllowWrite                                                 bool
	Labels                                                                pageLabels
}

func browserPageData(c *gin.Context) pageData {
	locale, _ := c.Cookie("NEXT_LOCALE")
	if _, ok := pageTranslations[locale]; !ok {
		language := strings.ToLower(c.GetHeader("Accept-Language"))
		switch {
		case strings.HasPrefix(language, "zh"):
			locale = "zh-CN"
		case strings.HasPrefix(language, "ru"):
			locale = "ru"
		default:
			locale = "en"
		}
	}
	return pageData{Language: locale, Labels: pageTranslations[locale], Nonce: randomCredential("")}
}

func showConsent(c *gin.Context, _ instance, client publicClient, request authorizationRequest, intent string) {
	data := browserPageData(c)
	data.Title, data.Detail = data.Labels.Consent, data.Labels.ConsentDetail
	data.Client, data.Scopes, data.Action, data.Intent = client.Name, request.Scope, authorizationPath, intent
	data.Redirect = request.RedirectURI
	read := readScopes(request.Scope)
	data.AllowRead, data.AllowWrite = read != "", read != request.Scope
	renderBrowserPage(c, data)
}

func showConnect(c *gin.Context, site instance) {
	if _, err := browserAdmin(c); err != nil {
		handleConsentLogin(c, err)
		return
	}
	data := browserPageData(c)
	data.Title, data.Detail = data.Labels.Connect, data.Labels.ConnectDetail
	data.Action = connectPath + "/revoke"
	data.Setup = fmt.Sprintf(data.Labels.Setup, "openflare-"+site.id()[3:11], site.mcpResource(), site.id())
	renderBrowserPage(c, data)
}

func renderBrowserPage(c *gin.Context, data pageData) {
	var output bytes.Buffer
	if err := browserPage.Execute(&output, data); err != nil {
		oauthInternalError(c, err)
		return
	}
	formAction := "'self'"
	if data.Intent != "" {
		redirect, err := url.Parse(data.Redirect)
		if err != nil || !validMCPRedirect(data.Redirect) {
			oauthError(c, http.StatusBadRequest, "invalid_redirect_uri")
			return
		}
		formAction += " " + redirect.Scheme + "://" + redirect.Host
	}
	c.Header("Content-Security-Policy", "default-src 'none'; script-src 'nonce-"+data.Nonce+"'; style-src 'unsafe-inline'; form-action "+formAction+"; frame-ancestors 'none'; base-uri 'none'")
	c.Data(http.StatusOK, "text/html; charset=utf-8", output.Bytes())
}

func revokeBrowserGrants(c *gin.Context, site instance) {
	if c.GetHeader("Origin") != site.origin {
		oauthError(c, http.StatusForbidden, "access_denied")
		return
	}
	user, err := browserAdmin(c)
	if err != nil {
		handleConsentLogin(c, err)
		return
	}
	if err := repository.RevokeUserAutomationGrants(c.Request.Context(), user.ID); err != nil {
		oauthInternalError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, connectPath)
}

// RegisterBrowser mounts setup instructions and account-wide grant revocation.
func RegisterBrowser(group *gin.RouterGroup) {
	group.GET("/connect", withInstance(showConnect))
	group.POST("/connect/revoke", withInstance(revokeBrowserGrants))
}
