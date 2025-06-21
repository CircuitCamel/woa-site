package rule

import (
	"net/http"
	"text/template"
	"warofages/internal/util"
)

func RulesHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/rules/index.html",
		"static/templates/footer.html",
	)
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}
	tmpl.ExecuteTemplate(w, "base", nil)
}
