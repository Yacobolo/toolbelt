package netsuite

import (
	"strings"
	"unicode"
)

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, character := range value {
		var output rune
		switch {
		case character >= 'a' && character <= 'z':
			output = character
		case character >= '0' && character <= '9':
			output = character
		case character == '\'' || character == '`':
			continue
		case character > unicode.MaxASCII:
			output = transliterate(character)
		default:
			output = '-'
		}
		if output == 0 {
			continue
		}
		if output == '-' {
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
			continue
		}
		builder.WriteRune(output)
		lastDash = false
	}
	return strings.Trim(builder.String(), "-")
}

func transliterate(character rune) rune {
	switch character {
	case 'à', 'á', 'â', 'ã', 'ä', 'å', 'ā', 'ă', 'ą':
		return 'a'
	case 'ç', 'ć', 'č':
		return 'c'
	case 'ď':
		return 'd'
	case 'è', 'é', 'ê', 'ë', 'ē', 'ĕ', 'ė', 'ę', 'ě':
		return 'e'
	case 'ì', 'í', 'î', 'ï', 'ī', 'ĭ', 'į':
		return 'i'
	case 'ñ', 'ń':
		return 'n'
	case 'ò', 'ó', 'ô', 'õ', 'ö', 'ø', 'ō':
		return 'o'
	case 'ù', 'ú', 'û', 'ü', 'ū':
		return 'u'
	case 'ý', 'ÿ':
		return 'y'
	case 'ž', 'ź', 'ż':
		return 'z'
	case 'ß':
		return 's'
	case '’', '‘', 'ʼ':
		return 0
	default:
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			return 0
		}
		return '-'
	}
}
