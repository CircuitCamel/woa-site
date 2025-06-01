package woa

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"text/template"
	"warofages/internal/util"
)

func SessionHandler(w http.ResponseWriter, r *http.Request) {
	sessionDetailHandler(w, r)
}

func sessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := getSessions()
	if err != nil {
		return
	}
	tmpl, err := template.ParseFiles("website/sessions/index.html")
	if err != nil {
		return
	}
	tmpl.Execute(w, sessions)
}

func sessionDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		// No ID provided — serve sessions list
		sessions(w, r)
		return
	}

	sessionID, _ := strconv.Atoi(id)

	tmpl, err := template.ParseFiles("./website/sessions/session.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	sessions, _ := getSessions()

	var selected Session
	for _, a := range sessions {
		if a.ID == sessionID {
			selected = a
		}
	}

	databytes, _ := os.ReadFile(selected.Path)

	selected.Data = util.MdToHTML(databytes)

	tmpl.Execute(w, selected)
}

func getSessions() ([]Session, error) {
	files, err := filepath.Glob("./md/sessions/*.md")
	if err != nil {
		return nil, err
	}
	result := make([]Session, len(files))
	for i, v := range files {
		result[i] = Session{ID: i, Path: v}
	}
	return result, nil
}
