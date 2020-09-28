package version

import (
	"fmt"
	"unicode"
)

/*
Full Parsley specification for parsing the dependency specification.
Parsing is described in detail at: https://www.python.org/dev/peps/pep-0508/

	wsp            = ' ' | '\t'
	version_cmp    = wsp* <'<=' | '<' | '!=' | '==' | '>=' | '>' | '~=' | '==='>
	version        = wsp* <( letterOrDigit | '-' | '_' | '.' | '*' | '+' | '!' )+>
	version_one    = version_cmp:op version:v wsp* -> (op, v)
	version_many   = version_one:v1 (wsp* ',' version_one)*:v2 -> [v1] + v2
	versionspec    = ('(' version_many:v ')' ->v) | version_many
	urlspec        = '@' wsp* <URI_reference>
	marker_op      = version_cmp | (wsp* 'in') | (wsp* 'not' wsp+ 'in')
	python_str_c   = (wsp | letter | digit | '(' | ')' | '.' | '{' | '}' |
					'-' | '_' | '*' | '#' | ':' | ';' | ',' | '/' | '?' |
					'[' | ']' | '!' | '~' | '`' | '@' | '$' | '%' | '^' |
					'&' | '=' | '+' | '|' | '<' | '>' )
	dquote         = '"'
	squote         = '\\''
	python_str     = (squote <(python_str_c | dquote)*>:s squote |
					dquote <(python_str_c | squote)*>:s dquote) -> s
	env_var        = ('python_version' | 'python_full_version' |
					'os_name' | 'sys_platform' | 'platform_release' |
					'platform_system' | 'platform_version' |
					'platform_machine' | 'platform_python_implementation' |
					'implementation_name' | 'implementation_version' |
					'extra' # ONLY when defined by a containing layer
					):varname -> lookup(varname)
	marker_var     = wsp* (env_var | python_str)
	marker_expr    = marker_var:l marker_op:o marker_var:r -> (o, l, r)
				   | wsp* '(' marker:m wsp* ')' -> m
	marker_and     = marker_expr:l wsp* 'and' marker_expr:r -> ('and', l, r)
				   | marker_expr:m -> m
	marker_or      = marker_and:l wsp* 'or' marker_and:r -> ('or', l, r)
				   | marker_and:m -> m
	marker         = marker_or
	quoted_marker  = ';' wsp* marker
	identifier_end = letterOrDigit | (('-' | '_' | '.' )* letterOrDigit)
	identifier     = < letterOrDigit identifier_end* >
	name           = identifier
	extras_list    = identifier:i (wsp* ',' wsp* identifier)*:ids -> [i] + ids
	extras         = '[' wsp* extras_list?:e wsp* ']' -> e
	name_req       = (name:n wsp* extras?:e wsp* versionspec?:v wsp* quoted_marker?:m
					 -> (n, e or [], v or [], m))
	url_req        = (name:n wsp* extras?:e wsp* urlspec:v (wsp+ | end) quoted_marker?:m
					 -> (n, e or [], v, m))
	specification  = wsp* ( url_req | name_req ):s wsp* -> s
	# The result is a tuple - name, list-of-extras,
	# list-of-version-constraints-or-a-url, marker-ast or None


	URI_reference = <URI | relative_ref>
	URI           = scheme ':' hier_part ('?' query )? ( '#' fragment)?
	hier_part     = ('//' authority path_abempty) | path_absolute | path_rootless | path_empty
	absolute_URI  = scheme ':' hier_part ( '?' query )?
	relative_ref  = relative_part ( '?' query )? ( '#' fragment )?
	relative_part = '//' authority path_abempty | path_absolute | path_noscheme | path_empty
	scheme        = letter ( letter | digit | '+' | '-' | '.')*
	authority     = ( userinfo '@' )? host ( ':' port )?
	userinfo      = ( unreserved | pct_encoded | sub_delims | ':')*
	host          = IP_literal | IPv4address | reg_name
	port          = digit*
	IP_literal    = '[' ( IPv6address | IPvFuture) ']'
	IPvFuture     = 'v' hexdig+ '.' ( unreserved | sub_delims | ':')+
	IPv6address   = (
					( h16 ':'){6} ls32
					| '::' ( h16 ':'){5} ls32
					| ( h16 )?  '::' ( h16 ':'){4} ls32
					| ( ( h16 ':')? h16 )? '::' ( h16 ':'){3} ls32
					| ( ( h16 ':'){0,2} h16 )? '::' ( h16 ':'){2} ls32
					| ( ( h16 ':'){0,3} h16 )? '::' h16 ':' ls32
					| ( ( h16 ':'){0,4} h16 )? '::' ls32
					| ( ( h16 ':'){0,5} h16 )? '::' h16
					| ( ( h16 ':'){0,6} h16 )? '::' )
	h16           = hexdig{1,4}
	ls32          = ( h16 ':' h16) | IPv4address
	IPv4address   = dec_octet '.' dec_octet '.' dec_octet '.' dec_octet
	nz            = ~'0' digit
	dec_octet     = (
					digit # 0-9
					| nz digit # 10-99
					| '1' digit{2} # 100-199
					| '2' ('0' | '1' | '2' | '3' | '4') digit # 200-249
					| '25' ('0' | '1' | '2' | '3' | '4' | '5') )# %250-255
	reg_name = ( unreserved | pct_encoded | sub_delims)*
	path = (
			path_abempty # begins with '/' or is empty
			| path_absolute # begins with '/' but not '//'
			| path_noscheme # begins with a non-colon segment
			| path_rootless # begins with a segment
			| path_empty ) # zero characters
	path_abempty  = ( '/' segment)*
	path_absolute = '/' ( segment_nz ( '/' segment)* )?
	path_noscheme = segment_nz_nc ( '/' segment)*
	path_rootless = segment_nz ( '/' segment)*
	path_empty    = pchar{0}
	segment       = pchar*
	segment_nz    = pchar+
	segment_nz_nc = ( unreserved | pct_encoded | sub_delims | '@')+
					# non-zero-length segment without any colon ':'
	pchar         = unreserved | pct_encoded | sub_delims | ':' | '@'
	query         = ( pchar | '/' | '?')*
	fragment      = ( pchar | '/' | '?')*
	pct_encoded   = '%' hexdig
	unreserved    = letter | digit | '-' | '.' | '_' | '~'
	reserved      = gen_delims | sub_delims
	gen_delims    = ':' | '/' | '?' | '#' | '(' | ')?' | '@'
	sub_delims    = '!' | '$' | '&' | '\\'' | '(' | ')' | '*' | '+' | ',' | ';' | '='
	hexdig        = digit | 'a' | 'A' | 'b' | 'B' | 'c' | 'C' | 'd' | 'D' | 'e' | 'E' | 'f' | 'F'
*/

// ErrURLNotSupported is returned when a URL is encountered in the dependency
// specification. However, URLs are not allowed when specifying dependants.
var ErrURLNotSupported = fmt.Errorf("url not supported")

// Dependency is a parsed dependency specification.
type Dependency struct {
	Name     string
	Versions []Requirement
	Extras   []string

	expr []Expr
}

// ParseDependency parses a dependency according to PEP 508.
// Read more: https://www.python.org/dev/peps/pep-0508/
func ParseDependency(input string) (*Dependency, error) {
	p := &parser{s: input}
	d := &Dependency{}

	p.skipWhitespace()
	name := p.expectFunc(identifier)
	if name == "" {
		return nil, fmt.Errorf("expected distribution name")
	}
	d.Name = name

	p.skipWhitespace()
	if p.peekRune() == '[' {
		extras, err := extras(p)
		if err != nil {
			return nil, err
		}
		d.Extras = extras
	}

	p.skipWhitespace()
	switch r := p.peekRune(); {
	case r == '(':
		p.next()

		var err error
		d.Versions, err = versionRequirements(p)
		if err != nil {
			return nil, err
		}

		if p.next() != ')' {
			return nil, fmt.Errorf("expected closing parenthesis")
		}
	case p.peek(comparisonOps...):
		var err error
		d.Versions, err = versionRequirements(p)
		if err != nil {
			return nil, err
		}
	case r == '@':
		return nil, ErrURLNotSupported
	case r == eof:
		return d, nil
	}

	p.skipWhitespace()
	if r := p.peekRune(); r == ';' {
		expr, err := environmentMarkers(p)
		if err != nil {
			return nil, err
		}
		d.expr = expr
	}

	p.skipWhitespace()
	if r := p.peekRune(); r != eof {
		return nil, fmt.Errorf("expected end of string, remaining: '%s'", p.s[p.pos:])
	}

	return d, nil
}

func ParseVersionRequirements(input string) ([]Requirement, error) {
	p := &parser{s: input}

	return versionRequirements(p)
}

func versionRequirement(p *parser) (Requirement, error) {
	p.skipWhitespace()
	op := p.expect(comparisonOps...)
	if op == "" {
		return Requirement{}, fmt.Errorf("expected version comparison operator")
	}

	p.skipWhitespace()
	versionString := p.expectFunc(isVersion)
	if versionString == "" {
		return Requirement{}, fmt.Errorf("expected valid version after comparison operator")
	}

	version, valid := Parse(versionString)
	if !valid {
		return Requirement{}, fmt.Errorf("invalid version '%s'", versionString)
	}

	return Requirement{
		Operator: op,
		Version:  version,
	}, nil
}

func versionRequirements(p *parser) ([]Requirement, error) {
	vrs := make([]Requirement, 0)
	for {
		vr, err := versionRequirement(p)
		if err != nil {
			return nil, err
		}
		vrs = append(vrs, vr)

		p.skipWhitespace()
		if r := p.peekRune(); r == ',' {
			p.next()
		} else if p.peek(comparisonOps...) {
			// Multiple version specifiers should be separated by comma but in
			// some cases a new version comparison operators begins right away.
			continue
		} else {
			return vrs, nil
		}
	}
}

func extras(p *parser) ([]string, error) {
	p.next() // consume '['

	extras := make([]string, 0)
	for {
		p.skipWhitespace()
		extra := p.expectFunc(identifier)
		if extra == "" {
			return nil, fmt.Errorf("expected extras")
		}
		extras = append(extras, extra)

		p.skipWhitespace()
		if r := p.peekRune(); r == ']' {
			p.next()
			return extras, nil
		} else if r == ',' {
			p.next()
		}
	}
}

var envVars = []string{
	"os_name",
	"sys_platform",
	"platform_machine",
	"platform_python_implementation",
	"platform_release",
	"platform_system",
	"platform_version",
	"python_version",
	"python_full_version",
	"implementation_name",
	"implementation_version",
	"extra",
}

var comparisonOps = []string{
	LessOrEqual,
	Less,
	Equal,
	NotEqual,
	GreaterOrEqual,
	Greater,
	CompatibleEqual,
	TripleEqual,
}

type marker struct {
	value string
	env   bool
}

func environmentMarker(p *parser) (marker, error) {
	p.skipWhitespace()
	switch r := p.peekRune(); r {
	case '"':
		p.next()
		v := p.expectFunc(func(r rune, _ int) bool {
			return isPythonString(r) || r == '\''
		})
		if p.next() != '"' {
			return marker{}, fmt.Errorf(`missing '"'`)
		}

		return marker{v, false}, nil
	case '\'':
		p.next()
		v := p.expectFunc(func(r rune, _ int) bool {
			return isPythonString(r) || r == '"'
		})
		if p.next() != '\'' {
			return marker{}, fmt.Errorf(`missing "'"`)
		}

		return marker{v, false}, nil
	default:
		env := p.expect(envVars...)
		if env == "" {
			return marker{}, fmt.Errorf("unknown environment marker, remaining: '%s'", p.s[p.pos:])
			// return marker{}, ErrUnknownEnvironmentMarker
		}

		return marker{env, true}, nil
	}
}

func environmentMarkerExpression(p *parser) (Expr, error) {
	p.skipWhitespace()
	if r := p.peekRune(); r == '(' {
		p.next()

		e, err := environmentMarkersDisjunction(p, nil)
		if err != nil {
			return nil, err
		}

		p.skipWhitespace()
		if p.next() != ')' {
			return nil, fmt.Errorf("expected closing parenthesis")
		}

		return e, nil
	}

	left, err := environmentMarker(p)
	if err != nil {
		return nil, fmt.Errorf("invalid left-hand environment marker: %v", err)
	}

	p.skipWhitespace()
	op := p.expect(append(comparisonOps, "in", "not in")...)
	if op == "" {
		return nil, fmt.Errorf("invalid operator")
	}

	right, err := environmentMarker(p)
	if err != nil {
		return nil, fmt.Errorf("invalid right-hand environment marker: %v", err)
	}

	return &exprMarker{
		op:    op,
		left:  left,
		right: right,
	}, nil
}

// environmentMarkersDisjunction parses a series of conjunctions separated by 'and'.
func environmentMarkersConjunction(p *parser, prev Expr) (Expr, error) {
	term, err := environmentMarkerExpression(p)
	if err != nil {
		return nil, err
	}

	if prev != nil {
		term = exprAnd{
			left:  prev,
			right: term,
		}
	}

	p.skipWhitespace()
	if p.expect("and") == "and" {
		return environmentMarkersConjunction(p, term)
	}

	return term, nil
}

// environmentMarkersDisjunction parses a series of conjunctions separated by 'or'.
// More than two terms are combined by nesting expressions.
// Execution order is preserved as long as the expressions are evaluated in
// depth-first order.
func environmentMarkersDisjunction(p *parser, prev Expr) (Expr, error) {
	term, err := environmentMarkersConjunction(p, nil)
	if err != nil {
		return nil, err
	}

	if prev != nil {
		term = exprOr{
			left:  prev,
			right: term,
		}
	}

	p.skipWhitespace()
	if p.expect("or") == "or" {
		return environmentMarkersDisjunction(p, term)
	}

	return term, nil
}

func environmentMarkers(p *parser) ([]Expr, error) {
	e := make([]Expr, 0)
	for {
		p.skipWhitespace()
		if r := p.peekRune(); r == ';' {
			p.next()
		} else {
			return e, nil
		}

		expr, err := environmentMarkersDisjunction(p, nil)
		if err != nil {
			return nil, err
		}
		e = append(e, expr)
	}
}

func identifier(r rune, i int) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || i > 0 && (r == '-' || r == '_' || r == '.')
}

func isPythonString(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsLetter(r) || unicode.IsDigit(r) ||
		r == '(' || r == ')' || r == '.' || r == '{' || r == '}' ||
		r == '-' || r == '_' || r == '*' || r == '#' || r == ':' ||
		r == ';' || r == ',' || r == '/' || r == '?' || r == '[' ||
		r == ']' || r == '!' || r == '~' || r == '`' || r == '@' ||
		r == '$' || r == '%' || r == '^' || r == '&' || r == '=' ||
		r == '+' || r == '|' || r == '<' || r == '>'
}

func isVersion(ch rune, i int) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '-' || ch == '_' || ch == '.' || ch == '*' || ch == '+' || ch == '!'
}
