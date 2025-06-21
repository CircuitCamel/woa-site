package session

import (
	"net/http"
	"strconv"
	"text/template"
	"warofages/internal/cache"
	"warofages/internal/util"
	"warofages/internal/woa"

	"github.com/gorilla/mux"
)

func SessionsHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/sessions/index.html",
		"static/templates/footer.html",
	)
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}
	tmpl.ExecuteTemplate(w, "base", cache.Sessions)
}

func SessionDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["session"]

	sessionID, _ := strconv.Atoi(id)

	tmpl, err := template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/sessions/session.html",
		"static/templates/footer.html",
	)
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}

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

	tmpl.ExecuteTemplate(w, "base", selected)
}
