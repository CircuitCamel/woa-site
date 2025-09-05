package util

import (
	"os"
	"strings"
)

func AddLinks(text string) string {
	result := text
	file, err := os.ReadFile(LoadConfig().LINKS)
	if err != nil {
		return text
	}
	links := linkArray(string(file))

	for ref, path := range links {
		parts := strings.Split(ref, " ")
		if len(parts) == 1 {
			result = strings.Replace(result, ref, mdLinkMaker(ref, path), 1)
		} else if len(parts) == 2 {
			result = nameReplace(parts, result, path)
		}
	}
	return result
}

func linkArray(file string) map[string]string {
	array := make(map[string]string)
	lines := strings.Split(file, "\n")

	for _, line := range lines {
		lineArr := strings.Split(line, "=")
		array[lineArr[0]] = lineArr[1]
	}

	return array
}

func nameReplace(parts []string, text string, path string) string {
	var result string
	result = strings.Replace(text, strings.Join(parts, " "), mdLinkMaker(strings.Join(parts, " "), path), 1)
	if result != text {
		return result
	}
	result = strings.Replace(text, parts[0], mdLinkMaker(parts[0], path), 1)
	if result != text {
		return result
	}
	result = strings.Replace(text, parts[1], mdLinkMaker(parts[1], path), 1)
	if result != text {
		return result
	}
	return result
}

func mdLinkMaker(ref, path string) string {
	return "[" + ref + "]" + "(" + path + ")"
}
