package util

import (
	"os"
	"strings"
)

func AddLinks(text string) string {
	var result string
	file, err := os.ReadFile(LoadConfig().LINKS)
	if err != nil {
		return text
	}
	links := LinkArray(string(file))

	for key, value := range links {
		result = strings.Replace(text, key, value, 1)
	}

	return result
}

func LinkArray(file string) map[string]string {
	array := make(map[string]string)
	lines := strings.Split(file, "\n")

	for _, line := range lines {
		lineArr := strings.Split(line, "=")
		array[lineArr[0]] = lineArr[1]
	}

	return array
}
