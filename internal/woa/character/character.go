package character

import (
	"net/http"
	"warofages/internal/cache"
	"warofages/internal/util"
	"warofages/internal/woa"

	"github.com/gorilla/mux"
)

func CharactersHandler(w http.ResponseWriter, r *http.Request) {
	cache.CharListTmpl.ExecuteTemplate(w, "base", cache.Characters)
}

func CharacterDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var selected woa.Character
	found := false
	for _, a := range cache.Characters {
		if a.NamePath == name {
			selected = a
			found = true
		}
	}

	if !found {
		util.ErrPage(w, r, 404)
		return
	}

	cache.CharTmpl.ExecuteTemplate(w, "base", selected)
}
