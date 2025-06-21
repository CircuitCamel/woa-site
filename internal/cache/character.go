package cache

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"warofages/internal/util"
	"warofages/internal/woa"
)

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
