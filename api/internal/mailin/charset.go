package mailin

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Jeux de caracteres d'un e-mail. UTF-8 domine ; restent les messageries qui
// ecrivent encore en Latin-1, en Latin-9 ou en Windows-1252 (Outlook). Les
// trois tiennent en une table de quelques lignes : pas de quoi importer
// golang.org/x/text.

// windows1252 donne les caracteres de 0x80 a 0x9F, la seule plage ou
// Windows-1252 differe de Latin-1. Zero : position non attribuee.
var windows1252 = [32]rune{
	'€', 0, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0, 'Ž', 0,
	0, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0, 'ž', 'Ÿ',
}

// latin9 : les huit positions ou Latin-9 differe de Latin-1.
var latin9 = map[byte]rune{
	0xA4: '€', 0xA6: 'Š', 0xA8: 'š', 0xB4: 'Ž', 0xB8: 'ž', 0xBC: 'Œ', 0xBD: 'œ', 0xBE: 'Ÿ',
}

// toUTF8 convertit un texte depuis son jeu de caracteres declare.
func toUTF8(content []byte, charset string) string {
	switch normalizeCharset(charset) {
	case "utf-8", "us-ascii", "":
		if utf8.Valid(content) {
			return string(content)
		}
		// Declare UTF-8 mais ne l'est pas : Windows-1252 est le suspect
		// habituel.
		return decodeSingleByte(content, "windows-1252")
	case "iso-8859-1", "windows-1252", "iso-8859-15":
		return decodeSingleByte(content, normalizeCharset(charset))
	default:
		if utf8.Valid(content) {
			return string(content)
		}
		return decodeSingleByte(content, "windows-1252")
	}
}

func normalizeCharset(charset string) string {
	c := strings.ToLower(strings.Trim(strings.TrimSpace(charset), `"`))
	switch c {
	case "utf8":
		return "utf-8"
	case "latin1", "iso8859-1", "iso_8859-1", "l1":
		return "iso-8859-1"
	case "cp1252", "windows1252", "x-cp1252":
		return "windows-1252"
	case "latin9", "iso8859-15", "iso_8859-15":
		return "iso-8859-15"
	case "ascii":
		return "us-ascii"
	}
	return c
}

func decodeSingleByte(content []byte, charset string) string {
	var b strings.Builder
	b.Grow(len(content))
	for _, c := range content {
		switch {
		case c < 0x80:
			b.WriteByte(c)
		case charset == "windows-1252" && c < 0xA0:
			if r := windows1252[c-0x80]; r != 0 {
				b.WriteRune(r)
			} else {
				b.WriteRune(rune(c))
			}
		case charset == "iso-8859-15":
			if r, ok := latin9[c]; ok {
				b.WriteRune(r)
			} else {
				b.WriteRune(rune(c))
			}
		default:
			b.WriteRune(rune(c))
		}
	}
	return b.String()
}

// charsetReader sert au decodage des en-tetes encodes (=?iso-8859-1?Q?…?=).
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	c := normalizeCharset(charset)
	switch c {
	case "utf-8", "us-ascii", "iso-8859-1", "windows-1252", "iso-8859-15":
		data, err := io.ReadAll(input)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader([]byte(toUTF8(data, c))), nil
	}
	return nil, fmt.Errorf("jeu de caracteres non pris en charge : %s", charset)
}
