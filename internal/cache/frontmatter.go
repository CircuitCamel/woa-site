package cache

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// parseFrontmatter reads a markdown file and splits it into its "---" delimited
// frontmatter (returned as a key/value map) and the markdown body that follows.
func parseFrontmatter(path string) (map[string]string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	meta := map[string]string{}
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
				meta[key] = val
			}
		} else if metaStarted {
			mdLines = append(mdLines, line)
		}
	}

	return meta, strings.Join(mdLines, "\n"), nil
}

// slug builds a URL-safe path segment from a name, falling back to the file's
// base name (without extension) when the name is empty.
func slug(name, path string) string {
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}
