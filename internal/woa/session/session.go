package session

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

func SessionsHandler(w http.ResponseWriter, r *http.Request) {
	sessions, err := getSessions()
	if err != nil {
		return
	}
	tmpl, err := template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/sessions/index.html",
		"static/templates/footer.html",
	)
	if err != nil {
		return
	}
	tmpl.ExecuteTemplate(w, "base", sessions)
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
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	sessions, _ := getSessions()

	var selected woa.Session
	for _, a := range sessions {
		if a.ID == sessionID {
			selected = a
		}
	}

	databytes, _ := os.ReadFile(selected.Path)

	selected.Body = util.MdToHTML(databytes)

	tmpl.ExecuteTemplate(w, "base", selected)
}

func getSessions() ([]woa.Session, error) {
	files, err := filepath.Glob("./md/sessions/*.md")
	if err != nil {
		return nil, err
	}
	result := make([]woa.Session, len(files))
	for i, v := range files {
		result[i] = woa.Session{ID: i + 1, Path: v}
	}
	return result, nil
}
