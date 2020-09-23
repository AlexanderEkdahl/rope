package version

import (
	"strings"
	"unicode/utf8"
)

var eof rune = -1

// simple string parser
type parser struct {
	s   string
	pos int
}

func (p *parser) expectFunc(f func(r rune, i int) bool) string {
	start := p.pos
	for i, r := range p.s[p.pos:] {
		if !f(r, i) {
			return p.s[start : start+i]
		}
		p.pos += utf8.RuneLen(r)
	}

	return p.s[start:]
}

func (p *parser) expect(ss ...string) string {
	for _, s := range ss {
		if strings.HasPrefix(p.s[p.pos:], s) {
			p.pos += len(s)
			return s
		}
	}

	return ""
}

func (p *parser) skipWhitespace() {
	for _, r := range p.s[p.pos:] {
		if r != ' ' && r != '\t' {
			break
		}
		p.pos += utf8.RuneLen(r)
	}
}

func (p *parser) peekRune() rune {
	for _, r := range p.s[p.pos:] {
		return r
	}

	return eof
}

func (p *parser) peek(ss ...string) bool {
	for _, s := range ss {
		if strings.HasPrefix(p.s[p.pos:], s) {
			return true
		}
	}
	return false
}

func (p *parser) next() rune {
	for _, r := range p.s[p.pos:] {
		p.pos += utf8.RuneLen(r)
		return r
	}

	return eof
}
