package item

import (
	"net/http"
	"warofages/internal/cache"
	"warofages/internal/util"
	"warofages/internal/woa"

	"github.com/gorilla/mux"
)

func ItemsHandler(w http.ResponseWriter, r *http.Request) {
	cache.ItemListTmpl.ExecuteTemplate(w, "base", cache.Items)
}

func ItemDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["item"]

	var selected woa.Item
	found := false
	for _, a := range cache.Items {
		if a.NamePath == name {
			selected = a
			found = true
		}
	}

	if !found {
		util.ErrPage(w, r, 404)
		return
	}

	cache.ItemTmpl.ExecuteTemplate(w, "base", selected)
}
