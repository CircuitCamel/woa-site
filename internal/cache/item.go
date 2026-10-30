package cache

import (
	"path/filepath"
	"warofages/internal/util"
	"warofages/internal/woa"
)

func loadItemMarkdown(path string) (woa.Item, error) {
	meta, body, err := parseFrontmatter(path)
	if err != nil {
		return woa.Item{}, err
	}

	it := woa.Item{
		Name:       meta["Name"],
		Type:       meta["Type"],
		Rarity:     meta["Rarity"],
		Attunement: meta["Attunement"],
	}
	it.Body = util.MdToHTML([]byte(body))
	it.NamePath = slug(it.Name, path)
	return it, nil
}

func getItems() ([]woa.Item, error) {
	files, err := filepath.Glob("./md/items/*.md")
	if err != nil {
		return nil, err
	}
	result := make([]woa.Item, len(files))
	for i, file := range files {
		result[i], _ = loadItemMarkdown(file)
	}
	return result, nil
}
