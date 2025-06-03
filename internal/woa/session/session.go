package session

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"text/template"
	"warofages/internal/util"
	"warofages/internal/woa"
)

func SessionHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		// No ID provided — serve sessions list
		sessions(w, r)
		return
	} else {
		sessionDetailHandler(w, r)
	}
}

func sessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := getSessions()
	if err != nil {
		return
	}
	tmpl, err := template.ParseFiles("static/sessions/index.html")
	if err != nil {
		return
	}
	tmpl.Execute(w, sessions)
}

func sessionDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")

	sessionID, _ := strconv.Atoi(id)

	tmpl, err := template.ParseFiles("./static/sessions/session.html")
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

	tmpl.Execute(w, selected)
}

func getSessions() ([]woa.Session, error) {
	files, err := filepath.Glob("./md/sessions/*.md")
	if err != nil {
		return nil, err
	}
	result := make([]woa.Session, len(files))
	for i, v := range files {
		result[i] = woa.Session{ID: i, Path: v}
	}
	return result, nil
}
