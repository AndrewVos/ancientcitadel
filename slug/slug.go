package slug

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func Slug(id int, text string) string {
	text = strings.ToLower(text)

	nonWordReplacer := regexp.MustCompile("[^\\w ]")
	text = nonWordReplacer.ReplaceAllString(text, "")

	text = strings.TrimSpace(text)

	spaceReplacer := regexp.MustCompile("\\ +")
	text = spaceReplacer.ReplaceAllString(text, "-")

	return fmt.Sprintf("%v-%v", id, text)
}

func Parse(slug string) (int, error) {
	idFinder := regexp.MustCompile("^\\d+")
	id, err := strconv.Atoi(idFinder.FindString(slug))
	return id, err
}
