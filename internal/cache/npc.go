package cache

import (
	"path/filepath"
	"warofages/internal/util"
	"warofages/internal/woa"
)

func loadNPCMarkdown(path string) (woa.NPC, error) {
	meta, body, err := parseFrontmatter(path)
	if err != nil {
		return woa.NPC{}, err
	}

	n := woa.NPC{
		Name:       meta["Name"],
		Race:       meta["Race"],
		Occupation: meta["Occupation"],
		Location:   meta["Location"],
		Status:     meta["Status"],
	}
	n.Body = util.MdToHTML([]byte(body))
	n.NamePath = slug(n.Name, path)
	return n, nil
}

func getNPCs() ([]woa.NPC, error) {
	files, err := filepath.Glob("./md/npcs/*.md")
	if err != nil {
		return nil, err
	}
	result := make([]woa.NPC, len(files))
	for i, file := range files {
		result[i], _ = loadNPCMarkdown(file)
	}
	return result, nil
}
