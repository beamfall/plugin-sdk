package catalogindex

import (
	"errors"
	"fmt"
	"strings"
)

// maxSPDXLength bounds expression size so parsing stays cheap on hostile
// input.
const maxSPDXLength = 512

// checkSPDX accepts a syntactically valid SPDX licence expression (SPDX 2.3
// Annex D) written in canonical form: upper-case AND / OR / WITH, single
// spaces between terms, no space inside parentheses. NONE and NOASSERTION
// are rejected as ambiguous. It does not decide whether a licence identifier
// is acceptable; the signed policy does that by exact match.
func checkSPDX(expr string) error {
	if expr == "" {
		return errors.New("empty SPDX expression")
	}
	if len(expr) > maxSPDXLength {
		return errors.New("SPDX expression too long")
	}
	tokens, err := tokenizeSPDX(expr)
	if err != nil {
		return err
	}
	if canonical := renderSPDX(tokens); canonical != expr {
		return fmt.Errorf("SPDX expression is not canonical; want %q", canonical)
	}
	p := spdxParser{tokens: tokens}
	if err := p.parseOr(); err != nil {
		return err
	}
	if p.pos != len(p.tokens) {
		return fmt.Errorf("unexpected %q", p.tokens[p.pos])
	}
	return nil
}

func tokenizeSPDX(expr string) ([]string, error) {
	var tokens []string
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			tokens = append(tokens, word.String())
			word.Reset()
		}
	}
	for _, r := range expr {
		switch {
		case r == '(' || r == ')':
			flush()
			tokens = append(tokens, string(r))
		case r == ' ':
			flush()
		case r < 0x21 || r > 0x7e:
			return nil, fmt.Errorf("invalid character %q in SPDX expression", r)
		default:
			word.WriteRune(r)
		}
	}
	flush()
	return tokens, nil
}

func renderSPDX(tokens []string) string {
	var b strings.Builder
	for i, tok := range tokens {
		if i > 0 && tokens[i-1] != "(" && tok != ")" {
			b.WriteByte(' ')
		}
		b.WriteString(tok)
	}
	return b.String()
}

type spdxParser struct {
	tokens []string
	pos    int
}

func (p *spdxParser) peek() string {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return ""
}

func (p *spdxParser) parseOr() error {
	if err := p.parseAnd(); err != nil {
		return err
	}
	for p.peek() == "OR" {
		p.pos++
		if err := p.parseAnd(); err != nil {
			return err
		}
	}
	return nil
}

func (p *spdxParser) parseAnd() error {
	if err := p.parseWith(); err != nil {
		return err
	}
	for p.peek() == "AND" {
		p.pos++
		if err := p.parseWith(); err != nil {
			return err
		}
	}
	return nil
}

func (p *spdxParser) parseWith() error {
	if p.peek() == "(" {
		p.pos++
		if err := p.parseOr(); err != nil {
			return err
		}
		if p.peek() != ")" {
			return errors.New("unbalanced parenthesis")
		}
		p.pos++
		return nil
	}
	if err := p.licenseRef(); err != nil {
		return err
	}
	if p.peek() == "WITH" {
		p.pos++
		exception := p.peek()
		if !isSPDXIDString(strings.TrimPrefix(exception, "AdditionRef-")) || isSPDXReserved(exception) {
			return fmt.Errorf("invalid licence exception %q", exception)
		}
		p.pos++
	}
	return nil
}

func (p *spdxParser) licenseRef() error {
	tok := p.peek()
	if tok == "" {
		return errors.New("missing licence identifier")
	}
	p.pos++
	if isSPDXReserved(tok) {
		return fmt.Errorf("%q is not a licence identifier", tok)
	}
	switch upper := strings.ToUpper(tok); upper {
	case "NONE", "NOASSERTION":
		return fmt.Errorf("%s is not an acceptable licence assertion", tok)
	}
	if doc, ref, ok := strings.Cut(tok, ":"); ok {
		if !strings.HasPrefix(doc, "DocumentRef-") || !isSPDXIDString(strings.TrimPrefix(doc, "DocumentRef-")) {
			return fmt.Errorf("invalid document reference %q", tok)
		}
		tok = ref
		if !strings.HasPrefix(tok, "LicenseRef-") {
			return fmt.Errorf("invalid licence reference %q", tok)
		}
	}
	if rest, ok := strings.CutPrefix(tok, "LicenseRef-"); ok {
		if !isSPDXIDString(rest) {
			return fmt.Errorf("invalid licence reference %q", tok)
		}
		return nil
	}
	if !isSPDXIDString(strings.TrimSuffix(tok, "+")) {
		return fmt.Errorf("invalid licence identifier %q", tok)
	}
	return nil
}

func isSPDXReserved(tok string) bool {
	switch strings.ToUpper(tok) {
	case "AND", "OR", "WITH", "(", ")":
		return true
	}
	return false
}

func isSPDXIDString(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !isSPDXIDRune(r) {
			return false
		}
	}
	return true
}

// isSPDXIDRune reports whether r may appear in an SPDX idstring.
func isSPDXIDRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-'
}
