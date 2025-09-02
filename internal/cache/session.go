package cache

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"warofages/internal/util"
	"warofages/internal/woa"
)

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
	md = util.AddLinks(md)
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
