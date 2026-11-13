package vdf

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// maxDepth bounds object nesting so that hostile input cannot exhaust the stack.
const maxDepth = 10000

// SyntaxError describes malformed VDF input.
type SyntaxError struct {
	msg string
	// Line is the 1-based line of the input on which the error was detected.
	Line int
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("vdf: syntax error on line %d: %s", e.Line, e.msg)
}

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokString
	tokOpen
	tokClose
	tokCond
)

type token struct {
	kind tokenKind
	text string
	line int
}

func (t token) String() string {
	switch t.kind {
	case tokEOF:
		return "EOF"
	case tokOpen:
		return "'{'"
	case tokClose:
		return "'}'"
	case tokCond:
		return fmt.Sprintf("conditional %q", t.text)
	default:
		return fmt.Sprintf("string %q", t.text)
	}
}

// pair is a single key/value from a VDF object. A value is either a string or
// a nested object, as indicated by isObject.
type pair struct {
	key      string
	value    string
	object   object
	isObject bool
}

type object []pair

type scanner struct {
	r       *bufio.Reader
	line    int
	peeked  *token
	started bool
}

func newScanner(r io.Reader) *scanner {
	return &scanner{r: bufio.NewReader(r), line: 1}
}

func (s *scanner) errorf(line int, format string, a ...any) error {
	return &SyntaxError{msg: fmt.Sprintf(format, a...), Line: line}
}

func (s *scanner) readByte() (byte, error) {
	b, err := s.r.ReadByte()
	if err == nil && b == '\n' {
		s.line++
	}
	return b, err
}

func (s *scanner) peekByte() (byte, bool) {
	b, err := s.r.Peek(1)
	if err != nil {
		return 0, false
	}
	return b[0], true
}

func isSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	}
	return false
}

func (s *scanner) peek() (token, error) {
	if s.peeked == nil {
		t, err := s.scan()
		if err != nil {
			return token{}, err
		}
		s.peeked = &t
	}
	return *s.peeked, nil
}

func (s *scanner) next() (token, error) {
	if s.peeked != nil {
		t := *s.peeked
		s.peeked = nil
		return t, nil
	}
	return s.scan()
}

var bom = []byte{0xEF, 0xBB, 0xBF}

func (s *scanner) scan() (token, error) {
	if !s.started {
		s.started = true
		if b, _ := s.r.Peek(len(bom)); bytes.Equal(b, bom) {
			_, _ = s.r.Discard(len(bom))
		}
	}

	for {
		b, err := s.readByte()
		if errors.Is(err, io.EOF) {
			return token{kind: tokEOF, line: s.line}, nil
		} else if err != nil {
			return token{}, err
		}

		switch {
		case isSpace(b):
		case b == '/':
			if n, ok := s.peekByte(); ok && n == '/' {
				if err := s.skipLine(); err != nil {
					return token{}, err
				}
				continue
			}
			return s.scanBare(b)
		case b == '{':
			return token{kind: tokOpen, line: s.line}, nil
		case b == '}':
			return token{kind: tokClose, line: s.line}, nil
		case b == '"':
			return s.scanQuoted()
		case b == '[':
			return s.scanCond()
		default:
			return s.scanBare(b)
		}
	}
}

func (s *scanner) skipLine() error {
	for {
		b, err := s.readByte()
		if errors.Is(err, io.EOF) || b == '\n' {
			return nil
		} else if err != nil {
			return err
		}
	}
}

// scanQuoted reads a quoted string whose opening quote has been consumed.
// The escapes \n, \t, \\ and \" are recognized; any other backslash is kept
// literally so that unescaped Windows paths survive.
func (s *scanner) scanQuoted() (token, error) {
	var (
		line = s.line
		buf  []byte
	)
	for {
		b, err := s.readByte()
		if errors.Is(err, io.EOF) {
			return token{}, s.errorf(line, "unterminated string")
		} else if err != nil {
			return token{}, err
		}

		switch b {
		case '"':
			return token{kind: tokString, text: string(buf), line: line}, nil
		case '\\':
			c, err := s.readByte()
			if errors.Is(err, io.EOF) {
				return token{}, s.errorf(line, "unterminated string")
			} else if err != nil {
				return token{}, err
			}

			switch c {
			case 'n':
				buf = append(buf, '\n')
			case 't':
				buf = append(buf, '\t')
			case '\\', '"':
				buf = append(buf, c)
			default:
				buf = append(buf, '\\', c)
			}
		default:
			buf = append(buf, b)
		}
	}
}

// scanBare reads an unquoted token, which ends at whitespace, a brace or a quote.
func (s *scanner) scanBare(first byte) (token, error) {
	var (
		line = s.line
		buf  = []byte{first}
	)
	for {
		b, ok := s.peekByte()
		if !ok || isSpace(b) || b == '{' || b == '}' || b == '"' {
			return token{kind: tokString, text: string(buf), line: line}, nil
		}
		_, _ = s.readByte()
		buf = append(buf, b)
	}
}

// scanCond reads a platform conditional such as [$WIN32] whose '[' has been consumed.
func (s *scanner) scanCond() (token, error) {
	var (
		line = s.line
		buf  []byte
	)
	for {
		b, err := s.readByte()
		if errors.Is(err, io.EOF) {
			return token{}, s.errorf(line, "unterminated conditional")
		} else if err != nil {
			return token{}, err
		}

		if b == ']' {
			return token{kind: tokCond, text: string(buf), line: line}, nil
		}
		buf = append(buf, b)
	}
}

// skipCond discards platform conditionals. They are accepted but not evaluated.
func (s *scanner) skipCond() error {
	for {
		t, err := s.peek()
		if err != nil {
			return err
		}
		if t.kind != tokCond {
			return nil
		}
		s.peeked = nil
	}
}

// parseDocument reads one VDF document. A document is either a braced object
// or, as in files written by Steam, a sequence of key/value pairs that runs to EOF.
// It returns io.EOF if the input holds no tokens.
func (s *scanner) parseDocument() (object, error) {
	t, err := s.peek()
	if err != nil {
		return nil, err
	}

	switch t.kind {
	case tokEOF:
		return nil, io.EOF
	case tokOpen:
		s.peeked = nil
		return s.parseObject(t.line, true, 1)
	default:
		return s.parseObject(t.line, false, 1)
	}
}

func (s *scanner) parseObject(openLine int, braced bool, depth int) (object, error) {
	if depth > maxDepth {
		return nil, s.errorf(openLine, "exceeded max depth of %d", maxDepth)
	}

	obj := object{}
	for {
		t, err := s.next()
		if err != nil {
			return nil, err
		}

		switch t.kind {
		case tokEOF:
			if braced {
				return nil, s.errorf(t.line, "unexpected EOF, missing '}' for object opened on line %d", openLine)
			}
			return obj, nil
		case tokClose:
			if !braced {
				return nil, s.errorf(t.line, "unexpected '}'")
			}
			return obj, nil
		case tokString:
			key := t.text
			if err := s.skipCond(); err != nil {
				return nil, err
			}

			v, err := s.next()
			if err != nil {
				return nil, err
			}

			switch v.kind {
			case tokString:
				obj = append(obj, pair{key: key, value: v.text})
			case tokOpen:
				child, err := s.parseObject(v.line, true, depth+1)
				if err != nil {
					return nil, err
				}
				obj = append(obj, pair{key: key, object: child, isObject: true})
			default:
				return nil, s.errorf(v.line, "unexpected %s looking for value of key %q", v, key)
			}

			if err := s.skipCond(); err != nil {
				return nil, err
			}
		default:
			return nil, s.errorf(t.line, "unexpected %s looking for key", t)
		}
	}
}
