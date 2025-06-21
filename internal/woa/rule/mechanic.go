package rule

import (
	"net/http"
	"os"
	"text/template"
	"warofages/internal/cache"
	"warofages/internal/util"
	"warofages/internal/woa"

	"github.com/gorilla/mux"
)

func MechanicsHandler(w http.ResponseWriter, r *http.Request) {
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
	tmpl.ExecuteTemplate(w, "base", cache.Mechanics)
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

	var selected woa.Rule
	found := false
	for _, a := range cache.Mechanics {
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
