package rule

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"text/template"
	"warofages/internal/util"
	"warofages/internal/woa"
)

func RulesHandler(w http.ResponseWriter, r *http.Request) {
	rule := r.URL.Query().Get("rule")
	if rule == "" {
		rulesMainPage(w, r)
		return
	} else {
		loadRule(w, r)
	}
}

func rulesMainPage(w http.ResponseWriter, r *http.Request) {
	rules, err := getRules()
	if err != nil {
		return
	}
	tmpl, err := template.ParseFiles("static/rules/index.html")
	if err != nil {
		return
	}
	tmpl.Execute(w, rules)
}

func loadRule(w http.ResponseWriter, r *http.Request) {
	rule := r.URL.Query().Get("rule")

	ruleID, _ := strconv.Atoi(rule)

	tmpl, err := template.ParseFiles("static/rules/rule.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	rules, _ := getRules()

	var selected woa.Rule
	for _, a := range rules {
		if a.ID == ruleID {
			selected = a
		}
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
