package review

import "strings"

// sanitizeLines returns the source split into lines with the *contents* of
// comments (// and /* */), string literals, char literals, and text blocks
// replaced by spaces. Delimiters and all code outside them are preserved, and
// the line count and per-line length are unchanged, so a regex rule can match
// real code without firing on a banned token that only appears inside a comment
// or a string (e.g. an "import org.zalando.problem" mentioned in a Javadoc, or
// a "?" inside a @Schema description). This is the single biggest precision
// lever for the lexical rules.
func sanitizeLines(src []byte) []string {
	const (
		normal = iota
		lineComment
		blockComment
		str
		chr
		textBlock
	)
	state := normal
	out := make([]byte, 0, len(src))
	put := func(b byte) { out = append(out, b) }
	// blank replaces a byte with a space, but keeps newlines/tabs so line
	// indices and (roughly) columns stay aligned with the raw source.
	blank := func(b byte) {
		switch b {
		case '\n', '\t', '\r':
			out = append(out, b)
		default:
			out = append(out, ' ')
		}
	}

	i, n := 0, len(src)
	for i < n {
		b := src[i]
		switch state {
		case normal:
			switch {
			case b == '"' && i+2 < n && src[i+1] == '"' && src[i+2] == '"':
				put('"')
				put('"')
				put('"')
				i += 3
				state = textBlock
			case b == '/' && i+1 < n && src[i+1] == '/':
				blank(b)
				blank(b)
				i += 2
				state = lineComment
			case b == '/' && i+1 < n && src[i+1] == '*':
				blank(b)
				blank(b)
				i += 2
				state = blockComment
			case b == '"':
				put(b)
				i++
				state = str
			case b == '\'':
				put(b)
				i++
				state = chr
			default:
				put(b)
				i++
			}
		case lineComment:
			if b == '\n' {
				put(b)
				state = normal
			} else {
				blank(b)
			}
			i++
		case blockComment:
			if b == '*' && i+1 < n && src[i+1] == '/' {
				blank(b)
				blank(b)
				i += 2
				state = normal
			} else {
				blank(b)
				i++
			}
		case str:
			switch {
			case b == '\\' && i+1 < n:
				blank(b)
				blank(src[i+1])
				i += 2
			case b == '"':
				put(b)
				i++
				state = normal
			case b == '\n': // unterminated literal; recover at line end
				put(b)
				i++
				state = normal
			default:
				blank(b)
				i++
			}
		case chr:
			switch {
			case b == '\\' && i+1 < n:
				blank(b)
				blank(src[i+1])
				i += 2
			case b == '\'':
				put(b)
				i++
				state = normal
			case b == '\n':
				put(b)
				i++
				state = normal
			default:
				blank(b)
				i++
			}
		case textBlock:
			if b == '"' && i+2 < n && src[i+1] == '"' && src[i+2] == '"' {
				put('"')
				put('"')
				put('"')
				i += 3
				state = normal
			} else {
				blank(b) // keeps newlines inside the block
				i++
			}
		}
	}
	return strings.Split(string(out), "\n")
}
