package version

import (
	"fmt"
	"strings"
	"text/scanner"
	"unicode"
)

func versionIdentifier(ch rune, i int) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '-' || ch == '_' || ch == '.' || ch == '*' || ch == '+' || ch == '!'
}

func skipWhitespace(s *scanner.Scanner) {
	for s.Whitespace&(1<<uint(s.Peek())) != 0 {
		s.Next()
	}
}

func scanVersionCmp(s *scanner.Scanner) (string, error) {
	skipWhitespace(s)
	ch := s.Next()
	switch ch {
	case '=':
		if n := s.Next(); n != '=' {
			return "", fmt.Errorf("expected = after = in version comparator, got: %c", n)
		}
		if s.Peek() == '=' {
			return TripleEqual, nil
		}
		return Equal, nil
	case '<':
		if s.Peek() == '=' {
			s.Next()
			return LessOrEqual, nil
		}
		return Less, nil
	case '!':
		if n := s.Next(); n != '=' {
			return "", fmt.Errorf("expected = after ! in version comparator, got: %c", n)
		}
		return NotEqual, nil
	case '~':
		if n := s.Next(); n != '=' {
			return "", fmt.Errorf("expected = after ~ in version comparator, got: %c", n)
		}
		return CompatibleEqual, nil
	case '>':
		if s.Peek() == '=' {
			s.Next()
			return ">=", nil
		}
		return ">", nil
	default:
		return "", fmt.Errorf("expected version comparator, got: %c", ch)
	}
}

func scanVersionRequirement(s *scanner.Scanner) (VersionRequirement, error) {
	cmp, err := scanVersionCmp(s)
	if err != nil {
		return VersionRequirement{}, err
	}

	s.IsIdentRune = versionIdentifier
	if s.Scan() == scanner.EOF {
		return VersionRequirement{}, fmt.Errorf("expected version, got EOF")
	}

	// TODO: Improve version parsing to support cases such as:
	// - tensorflow (!=2.0.*,<3,>=1.15)
	// - avro-python3 (!=1.9.2.*,<2.0.0,>=1.8.1)
	versionString := strings.ReplaceAll(s.TokenText(), "*", "0")

	version, err := Parse(versionString)
	if err != nil {
		return VersionRequirement{}, err
	}

	return VersionRequirement{
		Operator: cmp,
		Version:  version,
	}, nil
}

func ScanVersionRequirements(s *scanner.Scanner) ([]VersionRequirement, error) {
	vrs := make([]VersionRequirement, 0)
	for {
		vr, err := scanVersionRequirement(s)
		if err != nil {
			return nil, err
		}
		vrs = append(vrs, vr)

		skipWhitespace(s)
		if s.Peek() == ',' {
			s.Next()
		} else {
			break
		}
	}

	return vrs, nil
}
