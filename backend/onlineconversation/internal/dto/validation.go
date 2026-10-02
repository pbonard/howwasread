package dto

import (
	"fmt"
	"unicode/utf8"
)

func (r CreateConversationRequest) Validate() error {
	return validateContents(r.Novel, r.ShortStory, r.Poem, r.Play, r.Film, r.WrittenBy, r.Description)
}

func (r UpdateConversationRequest) Validate() error {
	return validateContents(r.Novel, r.ShortStory, r.Poem, r.Play, r.Film, r.WrittenBy, r.Description)
}

func validateContents(novel, shortStory, poem, play, film, writtenBy, description string) error {
	for _, f := range []struct {
		name  string
		value string
		max   int
	}{
		{"novel", novel, maxTitleChars},
		{"shortStory", shortStory, maxTitleChars},
		{"poem", poem, maxTitleChars},
		{"play", play, maxTitleChars},
		{"film", film, maxTitleChars},
		{"writtenBy", writtenBy, maxTitleChars},
		{"description", description, maxDescriptionChars},
	} {
		if utf8.RuneCountInString(f.value) > f.max {
			return fmt.Errorf("%s must be at most %d characters", f.name, f.max)
		}
	}
	return nil
}
