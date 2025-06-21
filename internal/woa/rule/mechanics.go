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

func MechanicsHandler(w http.ResponseWriter, r *http.Request) {
	rules, err := getMechanics()
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}
	tmpl, err := template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/rules/mechanics/index.html",
		"static/templates/footer.html",
	)
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}
	tmpl.ExecuteTemplate(w, "base", rules)
}

func MechanicDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mechanic := vars["mechanic"]

	tmpl, err := template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/rules/mechanics/mechanic.html",
		"static/templates/footer.html",
	)
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}

	mechanics, _ := getMechanics()

	var selected woa.Rule
	found := false
	for _, a := range mechanics {
		if a.TitlePath == mechanic {
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

func getMechanics() ([]woa.Rule, error) {
	files, err := filepath.Glob("./md/rules/mechanics/*.md")
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
