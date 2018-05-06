package lexer

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

// Lexer turns graphql request and schema strings into tokens
type Lexer struct {
	// The full input string
	input string
	// An offset into the string in bytes
	start int
	// An offset into the string in runes
	startRunes int
	// An offset into the string in bytes
	end int
	// An offset into the string in runes
	endRunes int
	// the current line number
	line int
	// An offset into the string in rune
	lineStartRunes int

	peeked    bool
	peekToken Token
	peekError error

	lastToken Token
}

func New(input string) Lexer {
	return Lexer{
		input: input,
		line:  1,
	}
}

// take one rune from input and advance end
func (s *Lexer) peek() (rune, int) {
	return utf8.DecodeRuneInString(s.input[s.end:])
}

// take one byte from input and advance end. This is a bit faster than take, but be careful not to break unicode support
func (s *Lexer) takeByte() uint8 {
	r := s.input[s.end]
	s.end++
	s.endRunes++
	return r
}

// get the remaining input.
func (s *Lexer) get() string {
	if s.start > len(s.input) {
		return ""
	}
	return s.input[s.start:]
}

func (s *Lexer) makeToken(kind Type) (Token, error) {
	return Token{
		Kind:   kind,
		Start:  s.startRunes,
		End:    s.endRunes,
		Value:  s.input[s.start:s.end],
		Line:   s.line,
		Column: s.startRunes - s.lineStartRunes + 1,
	}, nil
}

func (s *Lexer) makeError(format string, args ...interface{}) (Token, error) {
	return Token{
		Kind:   Invalid,
		Start:  s.startRunes,
		End:    s.endRunes,
		Line:   s.line,
		Column: s.endRunes - s.lineStartRunes + 1,
	}, fmt.Errorf(format, args...)
}

func (s *Lexer) LastToken() Token {
	return s.lastToken
}

func (s *Lexer) PeekToken() Token {
	if !s.peeked {
		s.peekToken, s.peekError = s.ReadToken()
		s.peeked = true
	}

	return s.peekToken
}

// ReadToken gets the next token from the source starting at the given position.
//
// This skips over whitespace and comments until it finds the next lexable
// token, then lexes punctuators immediately or calls the appropriate helper
// function for more complicated tokens.
func (s *Lexer) ReadToken() (token Token, err error) {
	defer func() {
		s.lastToken = token
	}()

	if s.peeked {
		s.peeked = false
		return s.peekToken, s.peekError
	}
	s.ws()
	s.start = s.end
	s.startRunes = s.endRunes

	if s.end >= len(s.input) {
		return s.makeToken(EOF)
	}
	r := s.input[s.start]
	s.end++
	s.endRunes++
	switch r {
	case '!':
		return s.makeToken(Bang)
	case '#':
		s.readComment()
		return s.ReadToken()
	case '$':
		return s.makeToken(Dollar)
	case '&':
		return s.makeToken(Amp)
	case '(':
		return s.makeToken(ParenL)
	case ')':
		return s.makeToken(ParenR)
	case '.':
		if len(s.input) > s.start+2 && s.input[s.start:s.start+3] == "..." {
			s.end += 2
			s.endRunes += 2
			return s.makeToken(Spread)
		}
	case ':':
		return s.makeToken(Colon)
	case '=':
		return s.makeToken(Equals)
	case '@':
		return s.makeToken(At)
	case '[':
		return s.makeToken(BracketL)
	case ']':
		return s.makeToken(BrackedR)
	case '{':
		return s.makeToken(BraceL)
	case '}':
		return s.makeToken(BraceR)
	case '|':
		return s.makeToken(Pipe)

	case '_', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
		return s.readName()

	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return s.readNumber()

	case '"':
		if len(s.input) > s.start+2 && s.input[s.start:s.start+3] == `"""` {
			return s.readBlockString()
		}

		return s.readString()
	}

	s.end--
	s.endRunes--

	if r < 0x0020 && r != 0x0009 && r != 0x000a && r != 0x000d {
		return s.makeError(`Cannot contain the invalid character "\u%04d"`, r)
	}

	if r == '\'' {
		return s.makeError(`Unexpected single quote character ('), did you mean to use a double quote (")?`)
	}

	return s.makeError(`Cannot parse the unexpected character "%s".`, string(r))
}

// ws reads from body starting at startPosition until it finds a non-whitespace
// or commented character, and updates the token end to include all whitespace
func (s *Lexer) ws() {
	for s.end < len(s.input) {
		switch s.input[s.end] {
		case '\t', ' ', ',':
			s.end++
			s.endRunes++
		case '\n':
			s.end++
			s.endRunes++
			s.line++
			s.lineStartRunes = s.endRunes
		case '\r':
			s.end++
			s.endRunes++
			s.line++
			s.lineStartRunes = s.endRunes
			// skip the following newline if its there
			if s.end < len(s.input) && s.input[s.end] == '\n' {
				s.end++
				s.endRunes++
			} else {
			}
			// byte order mark, given ws is hot path we aren't relying on the unicode package here.
		case 0xef:
			if s.end+2 < len(s.input) && s.input[s.end+1] == 0xBB && s.input[s.end+2] == 0xBF {
				s.end += 3
				s.endRunes++
			} else {
				return
			}
		default:
			return
		}
	}
}

// readComment from the input
//
// #[\u0009\u0020-\uFFFF]*
func (s *Lexer) readComment() (Token, error) {
	for s.end < len(s.input) {
		r, w := s.peek()

		// SourceCharacter but not LineTerminator
		if r > 0x001f || r == '\t' {
			s.end += w
			s.endRunes++
		} else {
			break
		}
	}

	return s.makeToken(Comment)
}

// readNumber from the input, either a float
// or an int depending on whether a decimal point appears.
//
// Int:   -?(0|[1-9][0-9]*)
// Float: -?(0|[1-9][0-9]*)(\.[0-9]+)?((E|e)(+|-)?[0-9]+)?
func (s *Lexer) readNumber() (Token, error) {
	float := false

	// backup to the first digit
	s.end--
	s.endRunes--

	s.acceptByte('-')

	if s.acceptByte('0') {
		if consumed := s.acceptDigits(); consumed != 0 {
			s.end -= consumed
			s.endRunes -= consumed
			return s.makeError("Invalid number, unexpected digit after 0: %s.", s.describeNext())
		}
	} else {
		if consumed := s.acceptDigits(); consumed == 0 {
			return s.makeError("Invalid number, expected digit but got: %s.", s.describeNext())
		}
	}

	if s.acceptByte('.') {
		float = true

		if consumed := s.acceptDigits(); consumed == 0 {
			return s.makeError("Invalid number, expected digit but got: %s.", s.describeNext())
		}
	}

	if s.acceptByte('e', 'E') {
		float = true

		s.acceptByte('-', '+')

		if consumed := s.acceptDigits(); consumed == 0 {
			return s.makeError("Invalid number, expected digit but got: %s.", s.describeNext())
		}
	}

	if float {
		return s.makeToken(Float)
	} else {
		return s.makeToken(Int)
	}
}

// acceptByte if it matches any of given bytes, returning true if it found anything
func (s *Lexer) acceptByte(bytes ...uint8) bool {
	if s.end >= len(s.input) {
		return false
	}

	for _, accepted := range bytes {
		if s.input[s.end] == accepted {
			s.end++
			s.endRunes++
			return true
		}
	}
	return false
}

// acceptByteRange accepts one byte inside the range provided, returning true if it found anything
func (s *Lexer) acceptByteRange(start uint8, end uint8) bool {
	if s.end < len(s.input) && s.input[s.end] >= start && s.input[s.end] <= end {
		s.end++
		s.endRunes++
		return true
	}
	return false
}

// acceptDigits from the input, returning the number of digits it found
func (s *Lexer) acceptDigits() int {
	consumed := 0
	for s.end < len(s.input) && s.input[s.end] >= '0' && s.input[s.end] <= '9' {
		s.end++
		s.endRunes++
		consumed++
	}

	return consumed
}

// describeNext peeks at the input and returns a human readable string. This should will alloc
// and should only be used in errors
func (s *Lexer) describeNext() string {
	if s.end < len(s.input) {
		return `"` + string(s.input[s.end]) + `"`
	}
	return "<EOF>"
}

// readString from the input
//
// "([^"\\\u000A\u000D]|(\\(u[0-9a-fA-F]{4}|["\\/bfnrt])))*"
func (s *Lexer) readString() (Token, error) {
	inputLen := len(s.input)

	// this buffer is lazily created only if there are escape characters.
	var buf *bytes.Buffer

	// skip the opening quote
	s.start++
	s.startRunes++

	for s.end < inputLen {
		r := s.input[s.end]
		if r == '\n' || r == '\r' {
			break
		}
		if r < 0x0020 && r != '\t' {
			return s.makeError(`Invalid character within String: "\u%04d".`, r)
		}
		switch r {
		default:
			var char = rune(r)
			var w = 1

			// skip unicode overhead if we are in the ascii range
			if r >= 127 {
				char, w = utf8.DecodeRuneInString(s.input[s.end:])
			}
			s.end += w
			s.endRunes++

			if buf != nil {
				buf.WriteRune(char)
			}

		case '"':
			t, err := s.makeToken(String)
			// the token should not include the quotes in its value, but should cover them in its position
			t.Start--
			t.End++

			if buf != nil {
				t.Value = buf.String()
			}

			// skip the close quote
			s.end++
			s.endRunes++

			return t, err

		case '\\':
			if s.end+1 >= inputLen {
				s.end++
				s.endRunes++
				return s.makeError(`Invalid character escape sequence.`)
			}

			if buf == nil {
				buf = bytes.NewBufferString(s.input[s.start:s.end])
			}

			escape := s.input[s.end+1]

			if escape == 'u' {
				if s.end+6 >= inputLen {
					s.end++
					s.endRunes++
					return s.makeError("Invalid character escape sequence: \\%s.", s.input[s.end:])
				}

				r, ok := unhex(s.input[s.end+2 : s.end+6])
				if !ok {
					s.end++
					s.endRunes++
					return s.makeError("Invalid character escape sequence: \\%s.", s.input[s.end:s.end+5])
				}
				buf.WriteRune(r)
				s.end += 6
				s.endRunes += 6
			} else {
				switch escape {
				case '"', '/', '\\':
					buf.WriteByte(escape)
				case 'b':
					buf.WriteByte('\b')
				case 'f':
					buf.WriteByte('\f')
				case 'n':
					buf.WriteByte('\n')
				case 'r':
					buf.WriteByte('\r')
				case 't':
					buf.WriteByte('\t')
				default:
					s.end += 1
					s.endRunes += 1
					return s.makeError("Invalid character escape sequence: \\%s.", string(escape))
				}
				s.end += 2
				s.endRunes += 2
			}
		}
	}

	return s.makeError("Unterminated string.")
}

// readBlockString from the input
//
// """("?"?(\\"""|\\(?!=""")|[^"\\]))*"""
func (s *Lexer) readBlockString() (Token, error) {
	inputLen := len(s.input)

	var buf bytes.Buffer

	// skip the opening quote
	s.start += 3
	s.startRunes += 3
	s.end += 2
	s.endRunes += 2

	for s.end < inputLen {
		r := s.input[s.end]

		// Closing triple quote (""")
		if r == '"' && s.end+3 <= inputLen && s.input[s.end:s.end+3] == `"""` {
			t, err := s.makeToken(BlockString)
			// the token should not include the quotes in its value, but should cover them in its position
			t.Start -= 3
			t.End += 3

			t.Value = blockStringValue(buf.String())

			// skip the close quote
			s.end += 3
			s.endRunes += 3

			return t, err
		}

		// SourceCharacter
		if r < 0x0020 && r != '\t' && r != '\n' && r != '\r' {
			return s.makeError(`Invalid character within String: "\u%04d".`, r)
		}

		if r == '\\' && s.end+4 <= inputLen && s.input[s.end:s.end+4] == `\"""` {
			buf.WriteString(`"""`)
			s.end += 4
			s.endRunes += 4
		} else if r == '\r' {
			if s.end+1 <= inputLen && s.input[s.end+1] == '\n' {
				s.end++
				s.endRunes++
			}

			buf.WriteByte('\n')
			s.end++
			s.endRunes++
		} else {
			var char = rune(r)
			var w = 1

			// skip unicode overhead if we are in the ascii range
			if r >= 127 {
				char, w = utf8.DecodeRuneInString(s.input[s.end:])
			}
			s.end += w
			s.endRunes++
			buf.WriteRune(char)
		}
	}

	return s.makeError("Unterminated string.")
}

func unhex(b string) (v rune, ok bool) {
	for _, c := range b {
		v <<= 4
		switch {
		case '0' <= c && c <= '9':
			v |= c - '0'
		case 'a' <= c && c <= 'f':
			v |= c - 'a' + 10
		case 'A' <= c && c <= 'F':
			v |= c - 'A' + 10
		default:
			return 0, false
		}
	}

	return v, true
}

// readName from the input
//
// [_A-Za-z][_0-9A-Za-z]*
func (s *Lexer) readName() (Token, error) {
	for s.end < len(s.input) {
		r, w := s.peek()

		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_' {
			s.end += w
			s.endRunes++
		} else {
			break
		}
	}

	return s.makeToken(Name)
}
