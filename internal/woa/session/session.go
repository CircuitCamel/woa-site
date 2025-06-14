package session

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"warofages/internal/util"
	"warofages/internal/woa"

	"github.com/gorilla/mux"
)

func SessionsHandler(w http.ResponseWriter, r *http.Request) {
	sessions, err := getSessions()
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}
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
		util.ErrPage(w, r, 500)
		return
	}

	sessions, _ := getSessions()

	var selected woa.Session
	found := false

	for _, a := range sessions {
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

func loadSessionMarkdown(path string) (woa.Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return woa.Session{}, err
	}
	defer file.Close()

	var s woa.Session
	var mdLines []string
	scanner := bufio.NewScanner(file)
	inMeta := false
	metaStarted := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "---" {
			if !metaStarted {
				inMeta = true
				metaStarted = true
				continue
			} else if inMeta {
				inMeta = false
				continue
			}
		}

		if inMeta {
			if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				switch key {
				case "Time":
					s.Time = val
				case "Place":
					s.Place = val
				}
			}
		} else if metaStarted {
			mdLines = append(mdLines, line)
		}
	}
	md := strings.Join(mdLines, "\n")
	s.Body = util.MdToHTML([]byte(md))
	return s, nil

}

func getSessions() ([]woa.Session, error) {
	files, err := filepath.Glob("./md/sessions/*.md")
	if err != nil {
		return nil, err
	}
	result := make([]woa.Session, len(files))
	for i, v := range files {
		result[i], _ = loadSessionMarkdown(v)
		result[i] = woa.Session{ID: i + 1, Path: v,
			Body: result[i].Body, Time: result[i].Time,
			Place: result[i].Place}
	}
	return result, nil
}
