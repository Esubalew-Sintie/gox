package naming

import (
	"fmt"
	"strings"
	"unicode"
)

type Name struct {
	Original string
	Pascal   string
	Camel    string
	Snake    string
	Kebab    string
	Plural   string
}

func Parse(value string) (*Name, error) {
	value = strings.TrimSpace(value)

	if value == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}

	words := splitWords(value)

	if len(words) == 0 {
		return nil, fmt.Errorf("invalid name %q", value)
	}

	for _, word := range words {
		if !isValidWord(word) {
			return nil, fmt.Errorf("invalid name %q", value)
		}
	}

	pascal := toPascal(words)
	camel := toCamel(words)
	snake := strings.Join(words, "_")
	kebab := strings.Join(words, "-")

	return &Name{
		Original: value,
		Pascal:   pascal,
		Camel:    camel,
		Snake:    snake,
		Kebab:    kebab,
		Plural:   pluralize(snake),
	}, nil
}

func splitWords(value string) []string {
	value = strings.TrimSpace(value)

	var words []string
	var current strings.Builder

	runes := []rune(value)
	n := len(runes)

	flush := func() {
		if current.Len() > 0 {
			words = append(words, strings.ToLower(current.String()))
			current.Reset()
		}
	}

	for i := 0; i < n; i++ {
		r := runes[i]

		switch {
		case r == '_' || r == '-' || r == ' ':
			flush()

		case unicode.IsUpper(r):
			// Start of a new word if:
			// - current buffer has content AND the previous char was lowercase
			// - OR next char is lowercase (ACRONYMWord → ACRONYM + Word)
			if current.Len() > 0 {
				prevLower := unicode.IsLower(runes[i-1])
				nextLower := i+1 < n && unicode.IsLower(runes[i+1])

				if prevLower || nextLower {
					flush()
				}
			}
			current.WriteRune(r)

		default:
			current.WriteRune(r)
		}
	}

	flush()

	return words
}


func isValidWord(word string) bool {
	for _, r := range word {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}

		return false
	}

	return true
}

func toPascal(words []string) string {
	var builder strings.Builder

	for _, word := range words {
		if word == "" {
			continue
		}

		runes := []rune(word)

		builder.WriteRune(unicode.ToUpper(runes[0]))
		builder.WriteString(string(runes[1:]))
	}

	return builder.String()
}

func toCamel(words []string) string {
	if len(words) == 0 {
		return ""
	}

	if len(words) == 1 {
		return words[0]
	}

	return strings.ToLower(words[0]) + toPascal(words[1:])
}

func pluralize(value string) string {
	switch {
	case strings.HasSuffix(value, "y"):
		return strings.TrimSuffix(value, "y") + "ies"

	case strings.HasSuffix(value, "s"):
		return value + "es"

	case strings.HasSuffix(value, "x"):
		return value + "es"

	case strings.HasSuffix(value, "ch"):
		return value + "es"

	case strings.HasSuffix(value, "sh"):
		return value + "es"

	default:
		return value + "s"
	}
}