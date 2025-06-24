package session

import (
	"net/http"
	"strconv"
	"warofages/internal/cache"
	"warofages/internal/util"
	"warofages/internal/woa"

	"github.com/gorilla/mux"
)

func SessionsHandler(w http.ResponseWriter, r *http.Request) {
	cache.SessionListTmpl.ExecuteTemplate(w, "base", cache.Sessions)
}

func SessionDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["session"]

	sessionID, _ := strconv.Atoi(id)

	var selected woa.Session
	found := false

	for _, a := range cache.Sessions {
		if a.ID == sessionID {
			selected = a
			found = true
		}
	}

	if !found {
		util.ErrPage(w, r, 404)
		return
	}

	cache.SessionTmpl.ExecuteTemplate(w, "base", selected)
}
