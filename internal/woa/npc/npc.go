package npc

import (
	"net/http"
	"warofages/internal/cache"
	"warofages/internal/util"
	"warofages/internal/woa"

	"github.com/gorilla/mux"
)

func NPCsHandler(w http.ResponseWriter, r *http.Request) {
	cache.NPCListTmpl.ExecuteTemplate(w, "base", cache.NPCs)
}

func NPCDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["npc"]

	var selected woa.NPC
	found := false
	for _, a := range cache.NPCs {
		if a.NamePath == name {
			selected = a
			found = true
		}
	}

	if !found {
		util.ErrPage(w, r, 404)
		return
	}

	cache.NPCTmpl.ExecuteTemplate(w, "base", selected)
}
