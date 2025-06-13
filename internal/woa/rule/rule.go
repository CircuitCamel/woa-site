package rule

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"text/template"
	"warofages/internal/util"
	"warofages/internal/woa"

	"github.com/gorilla/mux"
)

func RulesHandler(w http.ResponseWriter, r *http.Request) {
	rules, err := getRules()
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}
	tmpl, err := template.ParseFiles("static/rules/index.html")
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}
	tmpl.Execute(w, rules)
}

func RuleDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	rule := vars["rule"]

	ruleID, _ := strconv.Atoi(rule)

	tmpl, err := template.ParseFiles("static/rules/rule.html")
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}

	rules, _ := getRules()

	var selected woa.Rule
	found := false
	for _, a := range rules {
		if a.ID == ruleID {
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

	tmpl.Execute(w, selected)
}

func getRules() ([]woa.Rule, error) {
	files, err := filepath.Glob("./md/rules/*.md")
	if err != nil {
		return nil, err
	}
	result := make([]woa.Rule, len(files))
	for i, v := range files {
		result[i] = woa.Rule{ID: i + 1, Path: v}
	}
	return result, nil
}
