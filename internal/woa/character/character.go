package character

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"warofages/internal/util"
	"warofages/internal/woa"

	"github.com/gorilla/mux"
)

func CharactersHandler(w http.ResponseWriter, r *http.Request) {
	characters, err := getCharacters()
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}
	tmpl, err := template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/characters/index.html",
		"static/templates/footer.html",
	)
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}
	tmpl.ExecuteTemplate(w, "base", characters)
}

func CharacterDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	characterName := strings.ReplaceAll(name, "_", " ")

	tmpl, err := template.ParseFiles(
		"static/templates/head.html",
		"static/templates/titlebar.html",
		"static/characters/character.html",
		"static/templates/footer.html",
	)
	if err != nil {
		util.ErrPage(w, r, 500)
		return
	}

	characters, _ := getCharacters()

	var selected woa.Character
	found := false
	for _, a := range characters {
		if a.Name == characterName {
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

func loadCharacterMarkdown(path string) (woa.Character, error) {
	file, err := os.Open(path)
	if err != nil {
		return woa.Character{}, err
	}
	defer file.Close()

	var c woa.Character
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
				case "Name":
					c.Name = val
				case "Race":
					c.Race = val
				case "Class":
					c.Class = val
				case "Age":
					c.Age = val
				case "Level":
					c.Level = val
				}
			}
		} else if metaStarted {
			mdLines = append(mdLines, line)
		}
	}

	md := strings.Join(mdLines, "\n")
	c.Body = util.MdToHTML([]byte(md))
	c.NamePath = strings.ReplaceAll(c.Name, " ", "_")
	return c, nil
}

func getCharacters() ([]woa.Character, error) {
	files, err := filepath.Glob("./md/characters/*.md")
	if err != nil {
		return nil, err
	}
	result := make([]woa.Character, len(files))

	for i, file := range files {
		result[i], _ = loadCharacterMarkdown(file)
	}
	return result, nil
}
