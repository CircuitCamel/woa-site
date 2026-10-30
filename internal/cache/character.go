package cache

import (
	"path/filepath"
	"warofages/internal/util"
	"warofages/internal/woa"
)

func loadCharacterMarkdown(path string) (woa.Character, error) {
	meta, body, err := parseFrontmatter(path)
	if err != nil {
		return woa.Character{}, err
	}

	c := woa.Character{
		Name:  meta["Name"],
		Race:  meta["Race"],
		Class: meta["Class"],
		Age:   meta["Age"],
		Level: meta["Level"],
	}
	c.Body = util.MdToHTML([]byte(body))
	c.NamePath = slug(c.Name, path)
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
