package rule

import (
	"net/http"
	"os"
	"warofages/internal/cache"
	"warofages/internal/util"
	"warofages/internal/woa"

	"github.com/gorilla/mux"
)

func MechanicsHandler(w http.ResponseWriter, r *http.Request) {
	cache.MechanicListTmpl.ExecuteTemplate(w, "base", cache.Mechanics)
}

func MechanicDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	mechanic := vars["mechanic"]

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

	cache.MechanicTmpl.ExecuteTemplate(w, "base", selected)
}
