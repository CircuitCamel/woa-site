package cache

import (
	"path/filepath"
	"strings"
	"warofages/internal/woa"
)

func getTableRules() ([]woa.Rule, error) {
	files, err := filepath.Glob("./md/rules/table/*.md")
	if err != nil {
		return nil, err
	}
	result := make([]woa.Rule, len(files))
	for i, v := range files {
		title := strings.Split(filepath.Base(v), ".")[0]
		result[i] = woa.Rule{Path: v, Title: title, TitlePath: strings.ReplaceAll(title, " ", "-")}
	}
	return result, nil
}
