// Package templater extracts MCP prompt parameter declarations from Templater templates.
package templater

import (
	"regexp"
	"strings"
)

// Parameter is one <% tp.mcpTools.prompt("name", "description") %> declaration.
type Parameter struct {
	Name        string
	Description string
}

var startTag = regexp.MustCompile(`<%[*\-_]*`)
var endTag = regexp.MustCompile(`[\-_]*%>`)

// ParseParameters scans content for tp.mcpTools.prompt("arg", "optional description") calls inside Templater tags.
// It mirrors the upstream acorn-based parser but only supports the literal patterns used by obsidian-mcp-tools.
func ParseParameters(content string) []Parameter {
	parts := startTag.Split(content, -1)
	var out []Parameter
	for _, part := range parts {
		idx := endTag.FindStringIndex(part)
		if idx == nil {
			continue
		}
		code := strings.TrimSpace(part[:idx[0]])
		for _, p := range parsePromptCalls(code) {
			out = append(out, p)
		}
	}
	return out
}

func parsePromptCalls(code string) []Parameter {
	const needle = "tp.mcpTools.prompt"
	var res []Parameter
	search := code
	for {
		i := strings.Index(search, needle)
		if i < 0 {
			break
		}
		rest := search[i+len(needle):]
		rest = strings.TrimLeft(rest, " \t\n")
		if !strings.HasPrefix(rest, "(") {
			search = rest
			continue
		}
		args, after, ok := parseCallArgs(rest)
		if !ok {
			if len(rest) > 0 {
				search = rest[1:]
			} else {
				break
			}
			continue
		}
		if len(args) >= 1 {
			p := Parameter{Name: args[0]}
			if len(args) >= 2 {
				p.Description = args[1]
			}
			res = append(res, p)
		}
		search = after
	}
	return res
}

func parseCallArgs(s string) (args []string, tail string, ok bool) {
	if len(s) == 0 || s[0] != '(' {
		return nil, s, false
	}
	s = s[1:]
	for {
		s = strings.TrimLeft(s, " \t\n")
		if len(s) == 0 {
			return nil, s, false
		}
		if s[0] == ')' {
			return args, s[1:], true
		}
		arg, rest, ok2 := parseStringArg(s)
		if !ok2 {
			return nil, s, false
		}
		args = append(args, arg)
		s = strings.TrimLeft(rest, " \t\n")
		if len(s) == 0 {
			return nil, s, false
		}
		if s[0] == ',' {
			s = s[1:]
			continue
		}
		if s[0] == ')' {
			return args, s[1:], true
		}
		// trailing junk; stop
		return args, s, false
	}
}

func parseStringArg(s string) (value string, rest string, ok bool) {
	if len(s) == 0 {
		return "", s, false
	}
	q := s[0]
	if q != '"' && q != '\'' {
		return "", s, false
	}
	var b strings.Builder
	i := 1
	for i < len(s) {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		if c == byte(q) {
			return b.String(), s[i+1:], true
		}
		b.WriteByte(c)
		i++
	}
	return "", s, false
}
