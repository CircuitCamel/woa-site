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
		sessions(w)
		return
	} else {
		sessionDetailHandler(w, r)
	}
}

func sessions(w http.ResponseWriter) {
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

func sessionDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")

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
