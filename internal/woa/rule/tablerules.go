package rule

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"warofages/internal/util"
	"warofages/internal/woa"

	"github.com/gorilla/mux"
)

func TableRulesHandler(w http.ResponseWriter, r *http.Request) {
	rules, err := getTableRules()
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}
	tmpl, err := template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/rules/table/index.html",
		"static/templates/footer.html",
	)
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}
	tmpl.ExecuteTemplate(w, "base", rules)
}

func TableRuleDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tablerule := vars["tablerule"]

	tmpl, err := template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/rules/table/rule.html",
		"static/templates/footer.html",
	)
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}

	mechanics, _ := getTableRules()

	var selected woa.Rule
	found := false
	for _, a := range mechanics {
		if a.TitlePath == tablerule {
			selected = a
			found = true
		}
	}

	if !found {
		util.ErrPage(w, r, 404)
		return
	}

	databytes, _ := os.ReadFile(selected.Path)

	selected.Body = util.MdToHTML(databytes)

	tmpl.ExecuteTemplate(w, "base", selected)
}

func getTableRules() ([]woa.Rule, error) {
	files, err := filepath.Glob("./md/rules/table/*.md")
	if err != nil {
		return nil, err
	}
	result := make([]woa.Rule, len(files))
	for i, v := range files {
		title := strings.Split(filepath.Base(v), ".")[0]
		result[i] = woa.Rule{Path: v, Title: title, TitlePath: strings.ReplaceAll(title, " ", "_")}
	}
	return result, nil
}
