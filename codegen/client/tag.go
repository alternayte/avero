package client

import "strconv"

// lookupTag returns the value of one key of a struct tag.
//
// The parser is the one that the standard library states for a struct tag. The
// generator does not call reflect, so no package that a call path reaches
// imports it. See design rule 2.
func lookupTag(tag, key string) (string, bool) {
	for tag != "" {
		// Skip the leading spaces.
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			return "", false
		}
		// Read the name up to the colon.
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			return "", false
		}
		name := tag[:i]
		tag = tag[i+1:]
		// Read the quoted value.
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			return "", false
		}
		quoted := tag[:i+1]
		tag = tag[i+1:]
		if name == key {
			value, err := strconv.Unquote(quoted)
			if err != nil {
				return "", false
			}
			return value, true
		}
	}
	return "", false
}
