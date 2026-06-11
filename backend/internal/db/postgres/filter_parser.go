package postgres

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Token types
// ---------------------------------------------------------------------------

type filterTokenKind int

const (
	ftkAnd filterTokenKind = iota
	ftkOr
	ftkNot
	ftkLParen
	ftkRParen
	ftkTag       // t=<uuid>
	ftkMimeExact // m=<int>
	ftkMimeLike  // m~<pattern>
)

type filterToken struct {
	kind     filterTokenKind
	tagID    uuid.UUID // ftkTag
	untagged bool      // ftkTag with zero UUID → "file has no tags"
	mimeID   int16     // ftkMimeExact
	pattern  string    // ftkMimeLike
}

// ---------------------------------------------------------------------------
// AST nodes
// ---------------------------------------------------------------------------

// filterNode produces a parameterized SQL fragment.
// n is the index of the next available positional parameter ($n).
// Returns the fragment, the updated n, and the extended args slice.
type filterNode interface {
	toSQL(n int, args []any) (string, int, []any)
}

type andNode struct{ left, right filterNode }
type orNode struct{ left, right filterNode }
type notNode struct{ child filterNode }
type leafNode struct{ tok filterToken }

func (a *andNode) toSQL(n int, args []any) (string, int, []any) {
	ls, n, args := a.left.toSQL(n, args)
	rs, n, args := a.right.toSQL(n, args)
	return "(" + ls + " AND " + rs + ")", n, args
}

func (o *orNode) toSQL(n int, args []any) (string, int, []any) {
	ls, n, args := o.left.toSQL(n, args)
	rs, n, args := o.right.toSQL(n, args)
	return "(" + ls + " OR " + rs + ")", n, args
}

func (no *notNode) toSQL(n int, args []any) (string, int, []any) {
	cs, n, args := no.child.toSQL(n, args)
	return "(NOT " + cs + ")", n, args
}

func (l *leafNode) toSQL(n int, args []any) (string, int, []any) {
	switch l.tok.kind {
	case ftkTag:
		if l.tok.untagged {
			return "NOT EXISTS (SELECT 1 FROM data.file_tag ft WHERE ft.file_id = f.id)", n, args
		}
		s := fmt.Sprintf(
			"EXISTS (SELECT 1 FROM data.file_tag ft WHERE ft.file_id = f.id AND ft.tag_id = $%d)", n)
		return s, n + 1, append(args, l.tok.tagID)
	case ftkMimeExact:
		return fmt.Sprintf("f.mime_id = $%d", n), n + 1, append(args, l.tok.mimeID)
	case ftkMimeLike:
		// mt alias comes from the JOIN in the main file query (always present).
		return fmt.Sprintf("mt.name LIKE $%d", n), n + 1, append(args, l.tok.pattern)
	}
	panic("filterNode.toSQL: unknown leaf kind")
}

// ---------------------------------------------------------------------------
// Lexer
// ---------------------------------------------------------------------------

// lexFilter tokenises the DSL string {a,b,c,...} into filterTokens.
func lexFilter(dsl string) ([]filterToken, error) {
	dsl = strings.TrimSpace(dsl)
	if !strings.HasPrefix(dsl, "{") || !strings.HasSuffix(dsl, "}") {
		return nil, fmt.Errorf("filter DSL must be wrapped in braces: {…}")
	}
	inner := strings.TrimSpace(dsl[1 : len(dsl)-1])
	if inner == "" {
		return nil, nil
	}

	parts := strings.Split(inner, ",")
	tokens := make([]filterToken, 0, len(parts))

	for _, raw := range parts {
		p := strings.TrimSpace(raw)
		switch {
		case p == "&":
			tokens = append(tokens, filterToken{kind: ftkAnd})
		case p == "|":
			tokens = append(tokens, filterToken{kind: ftkOr})
		case p == "!":
			tokens = append(tokens, filterToken{kind: ftkNot})
		case p == "(":
			tokens = append(tokens, filterToken{kind: ftkLParen})
		case p == ")":
			tokens = append(tokens, filterToken{kind: ftkRParen})
		case strings.HasPrefix(p, "t="):
			id, err := uuid.Parse(p[2:])
			if err != nil {
				return nil, fmt.Errorf("filter: invalid tag UUID %q", p[2:])
			}
			tokens = append(tokens, filterToken{kind: ftkTag, tagID: id, untagged: id == uuid.Nil})
		case strings.HasPrefix(p, "m="):
			v, err := strconv.ParseInt(p[2:], 10, 16)
			if err != nil {
				return nil, fmt.Errorf("filter: invalid MIME ID %q", p[2:])
			}
			tokens = append(tokens, filterToken{kind: ftkMimeExact, mimeID: int16(v)})
		case strings.HasPrefix(p, "m~"):
			// The pattern value is passed as a query parameter, so no SQL injection risk.
			tokens = append(tokens, filterToken{kind: ftkMimeLike, pattern: p[2:]})
		default:
			return nil, fmt.Errorf("filter: unknown token %q", p)
		}
	}
	return tokens, nil
}

// ---------------------------------------------------------------------------
// Recursive-descent parser
// ---------------------------------------------------------------------------

type filterParser struct {
	tokens []filterToken
	pos    int
}

func (p *filterParser) peek() (filterToken, bool) {
	if p.pos >= len(p.tokens) {
		return filterToken{}, false
	}
	return p.tokens[p.pos], true
}

func (p *filterParser) next() filterToken {
	t := p.tokens[p.pos]
	p.pos++
	return t
}

// Grammar (standard NOT > AND > OR precedence):
//
//	expr     := or_expr
//	or_expr  := and_expr ('|' and_expr)*
//	and_expr := not_expr ('&' not_expr)*
//	not_expr := '!' not_expr | atom
//	atom     := '(' expr ')' | leaf

func (p *filterParser) parseExpr() (filterNode, error) { return p.parseOr() }

func (p *filterParser) parseOr() (filterNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for {
		t, ok := p.peek()
		if !ok || t.kind != ftkOr {
			break
		}
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &orNode{left, right}
	}
	return left, nil
}

func (p *filterParser) parseAnd() (filterNode, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for {
		t, ok := p.peek()
		if !ok || t.kind != ftkAnd {
			break
		}
		p.next()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &andNode{left, right}
	}
	return left, nil
}

func (p *filterParser) parseNot() (filterNode, error) {
	t, ok := p.peek()
	if ok && t.kind == ftkNot {
		p.next()
		child, err := p.parseNot() // right-recursive to allow !!x
		if err != nil {
			return nil, err
		}
		return &notNode{child}, nil
	}
	return p.parseAtom()
}

func (p *filterParser) parseAtom() (filterNode, error) {
	t, ok := p.peek()
	if !ok {
		return nil, fmt.Errorf("filter: unexpected end of expression")
	}
	if t.kind == ftkLParen {
		p.next()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		rp, ok := p.peek()
		if !ok || rp.kind != ftkRParen {
			return nil, fmt.Errorf("filter: expected ')'")
		}
		p.next()
		return expr, nil
	}
	switch t.kind {
	case ftkTag, ftkMimeExact, ftkMimeLike:
		p.next()
		return &leafNode{t}, nil
	default:
		return nil, fmt.Errorf("filter: unexpected token at position %d", p.pos)
	}
}

// ---------------------------------------------------------------------------
// Public entry point
// ---------------------------------------------------------------------------

// ParseFilter parses a filter DSL string into a parameterized SQL fragment.
//
// argStart is the 1-based index for the first $N placeholder; this lets the
// caller interleave filter parameters with other query parameters.
//
// Returns ("", argStart, nil, nil) for an empty or trivial DSL.
// SQL injection is structurally impossible: every user-supplied value is
// bound as a query parameter ($N), never interpolated into the SQL string.
func ParseFilter(dsl string, argStart int) (sql string, nextN int, args []any, err error) {
	dsl = strings.TrimSpace(dsl)
	if dsl == "" || dsl == "{}" {
		return "", argStart, nil, nil
	}
	toks, err := lexFilter(dsl)
	if err != nil {
		return "", argStart, nil, err
	}
	if len(toks) == 0 {
		return "", argStart, nil, nil
	}
	p := &filterParser{tokens: toks}
	node, err := p.parseExpr()
	if err != nil {
		return "", argStart, nil, err
	}
	if p.pos != len(p.tokens) {
		return "", argStart, nil, fmt.Errorf("filter: trailing tokens at position %d", p.pos)
	}
	sql, nextN, args = node.toSQL(argStart, nil)
	return sql, nextN, args, nil
}
