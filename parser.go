// Portions Copyright (c) 2013 The Go-IMAP Authors.
// https://code.google.com/p/go-imap/source/browse/LICENSE

package imap

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"io/ioutil"
)

// From: https://code.google.com/p/go-imap/source/browse/go1/imap/strings.go
const (
	nul  = 0x00
	ctl  = 0x20
	char = 0x80
)

const (
	crlf = "\r\n"
)

type Parser struct {
	w           Writer
	r           *bufio.Reader
	line        string
	pos         int
	isEOL       bool
	literal     bool
	literalSize int
	inSection   bool
	listDepth   int

	err error
}

type ProtocolError string

func (e ProtocolError) Error() string { return string(e) }

func ProtocolErrorf(format string, a ...interface{}) ProtocolError {
	return ProtocolError(fmt.Sprintf(format, a...))
}

func TypeMismatchError(expected string, context string) ProtocolError {
	return ProtocolErrorf("type mismatch: expected %s near %q", expected, context)
}

func InvalidTokenError(tokenType string, determinant byte, context string) ProtocolError {
	if determinant != 0 {
		return ProtocolErrorf("invalid byte %q in %s near %q", determinant, tokenType, context)
	} else {
		return ProtocolErrorf("invalid byte in %s near %q", tokenType, context)
	}
}

const maxTokenSize = 8000

func NewParser(c *Conn) *Parser {
	br := bufio.NewReaderSize(c.rwc, maxTokenSize)
	p := &Parser{
		w: c,
		r: br,

		isEOL: true,
	}
	return p
}

var (
	ErrTooLong          = ProtocolError("token too long")
	ErrLineTooShort     = ProtocolError("line too short")
	ErrNegativeAdvance  = ProtocolError("negative advance count")
	ErrAdvanceTooFar    = ProtocolError("advance count beyond input")
	ErrSeqNoOutOfBounds = ProtocolError("sequence number out of bounds")
)

// Adapted from: https://code.google.com/p/go-imap/source/browse/go1/imap/reader.go
//
// atomSpecials identifies ASCII characters that either may not appear in atoms
// or require special handling (ABNF: ATOM-CHAR).
var atomSpecials [char]bool

func init() {
	// atom-specials + '[' to provide special handling for BODY[...]
	s := []byte{'(', ')', '{', ' ', '%', '*', '"', '[', '\\', ']', '\x7F'}
	for c := byte(0); c < char; c++ {
		atomSpecials[c] = c < ctl || bytes.IndexByte(s, c) >= 0
	}
}

func (p *Parser) Err() error {
	return p.err
}

func (p *Parser) Valid() bool {
	return p.err == nil
}

func (p *Parser) accept(s string) bool {
	if !p.Valid() {
		return false
	}

	expected := strings.ToLower(s)
	l := len(expected)
	p.ensure(l)
	if !p.Valid() {
		if p.err == ErrLineTooShort {
			p.err = nil
		}
		return false
	}

	actual := strings.ToLower(p.Tail()[:l])

	if actual != expected {
		return false
	}

	p.advance(l)
	return true
}

func (p *Parser) Expect(s string) {
	if !p.Valid() {
		return
	}

	ok := p.accept(s)
	if p.Valid() && !ok {
		tail := p.Tail()
		if len(tail) > len(s) {
			tail = tail[:len(s)]
		}
		p.err = ProtocolErrorf("expected %q, got %q", s, tail)
	}
}

func (p *Parser) ReadListMailbox() string {
	return p.ReadStringWithExtra(map[byte]bool{'%': true, '*': true})
}

func (p *Parser) ReadSpace() {
	p.Expect(" ")
}

func (p *Parser) isValidDelimiter(c byte) bool {
	switch c {
	case ' ', '\r':
		return true
	case ']':
		return p.inSection
	case ')':
		return p.listDepth > 0
	default:
		return false
	}
}

func (p *Parser) ReadAtom() string {
	return p.ReadAtomWithExtra(map[byte]bool{})
}

func (p *Parser) ReadAtomWithExtra(extra map[byte]bool) string {
	data, n, flag := p.Tail(), 0, false
	for {
		p.ensure(n + 1)
		if !p.Valid() {
			return ""
		}

		data = p.Tail()
		if c := data[n]; c >= char || (atomSpecials[c] && !extra[c]) {
			switch c {
			case '\\':
				if n == 0 {
					flag = true
					n++
					continue // ABNF: flag (e.g. `\Seen`)
				}
			case '*':
				if n == 1 && flag {
					n++ // ABNF: flag-perm (`\*`), end of atom
				}
			}
			break // data[n] is a delimiter or an unexpected byte
		}
		n++
	}

	// Atom must have at least one character, two if it starts with a backslash
	if n < 2 && (n == 0 || flag) {
		p.err = InvalidTokenError("atom", 0, data)
		return ""
	}

	if !p.isValidDelimiter(data[n]) {
		p.err = InvalidTokenError("atom", data[n], data)
		return ""
	}

	bytes := data[:n]
	if norm := normalize(bytes); flag {
		bytes = norm
	}
	p.advance(n)
	return string(bytes)
}

// Private since it doesn't validate the ending delimiter
func (p *Parser) ReadInt() int {
	data, n := p.Tail(), 0
	for {
		p.ensure(n + 1)
		if !p.Valid() {
			return 0
		}

		if c := data[n]; c < '0' || c > '9' {
			break // data[n] is a delimiter or an unexpected byte
		}
		n++
	}

	// Integer must have at least one character
	if n < 1 {
		p.err = InvalidTokenError("integer", 0, data)
		return 0
	}

	str := string(data[:n])
	i, err := strconv.Atoi(str)
	if err != nil {
		p.err = err
		return 0
	}

	p.advance(n)
	if !p.Valid() {
		return 0
	}

	return i
}

func (p *Parser) Tail() string {
	return p.line[p.pos:]
}

func (p *Parser) Peek() byte {
	p.ensure(1)
	if p.available() < 1 {
		return 0
	}

	return p.line[p.pos]
}

func (p *Parser) ensure(ct int) {
	if p.isEOL {
		p.readLine()
		if !p.Valid() {
			return
		}
	}
	if p.available() < ct {
		p.err = ErrLineTooShort
	}
}

func (p *Parser) available() int {
	return len(p.Tail())
}

func (p *Parser) ReadLiteralPrefix() (size int, sync bool) {
	p.Expect("{")
	size = p.ReadInt()

	// RFC 2088 (LITERAL+): http://tools.ietf.org/html/rfc2088
	if p.accept("+") {
		sync = false
	} else {
		sync = true
	}

	p.Expect("}\r\n")
	return
}

func (p *Parser) ReadLiteral() string {
	stream := p.ReadLiteralStream()
	if !p.Valid() {
		return ""
	}

	bs, err := ioutil.ReadAll(stream)
	if err != nil {
		p.err = err
	}

	return string(bs)
}

func (p *Parser) ReadLiteralStream() io.Reader {
	size, sync := p.ReadLiteralPrefix()
	if !p.Valid() {
		return nil
	}

	if sync {
		p.w.Continuation("ready")
	}

	p.isEOL = true
	return io.LimitReader(p.r, int64(size))
}

func (p *Parser) advance(n int) {
	if n < 0 {
		panic(ErrNegativeAdvance)
	}
	if n > p.available() {
		panic(ErrAdvanceTooFar)
	}
	p.pos += n
	return
}

func (p *Parser) ReadDateTime() time.Time {
	s := p.ReadQuotedString()
	if !p.Valid() {
		return time.Unix(0, 0)
	}

	const longForm = "_2-Jan-2006 15:04:05 -0700" // ABNF: date-time
	t, err := time.Parse(longForm, s)
	if err != nil {
		p.err = err
		return time.Unix(0, 0)
	}

	return t
}

func (p *Parser) ReadDate() time.Time {
	s := p.ReadString()
	if !p.Valid() {
		return time.Unix(0, 0)
	}

	const longForm = "_2-Jan-2006" // ABNF: date
	t, err := time.Parse(longForm, s)
	if err != nil {
		p.err = err
		return time.Unix(0, 0)
	}

	return t
}

func (p *Parser) ReadQuotedString() string {
	c := p.Peek()
	if c != '"' {
		p.err = TypeMismatchError("quoted string", p.Tail())
		return ""
	}

	p.ensure(2) // minlength: ""
	if !p.Valid() {
		return ""
	}

	lastEscape := 1
	var buf bytes.Buffer
	data, n := p.Tail(), 1
Parse:
	for {
		p.ensure(n + 1)
		if !p.Valid() {
			return ""
		}

		data = p.Tail()
		c := data[n]
		switch c {
		case '\\':
			p.ensure(n + 1)
			if !p.Valid() {
				return ""
			}
			if data[n+1] == '\\' || data[n+1] == '"' {
				buf.WriteString(data[lastEscape:n])
				lastEscape = n + 1
			} else {
				p.err = InvalidTokenError("quoted string", data[n+1], data)
				return ""
			}
			n++
		case '"':
			break Parse
		}
		n++
	}

	if data[n] != '"' {
		p.err = InvalidTokenError("quoted string", data[n], data)
		return ""
	}
	n++

	if !p.isValidDelimiter(data[n]) {
		p.err = InvalidTokenError("atom", data[n], data)
		return ""
	}

	buf.Write([]byte(data[lastEscape : n-1]))

	p.advance(n)
	return buf.String()
}

func (p *Parser) ReadString() string {
	return p.ReadStringWithExtra(map[byte]bool{})
}

func (p *Parser) ReadAstring() string {
	return p.ReadStringWithExtra(map[byte]bool{']': true})
}

func (p *Parser) ReadStringWithExtra(extra map[byte]bool) string {
	c := p.Peek()
	if !p.Valid() {
		return ""
	}

	switch c {
	case '{':
		return p.ReadLiteral()
	case '"':
		return p.ReadQuotedString()
	default:
		return p.ReadAtomWithExtra(extra)
	}
}

func (p *Parser) ReadEOL() {
	p.Expect("\r\n")
	if p.Valid() {
		p.isEOL = true
	}
}

func (p *Parser) readLine() {
	p.pos = 0
	p.line, p.err = p.r.ReadString('\n')
	if len(p.line) > 0 {
		p.isEOL = false
	}
}

func (p *Parser) DiscardLine() {
	p.err = nil
	p.isEOL = true
}

func (p *Parser) String() string {
	return p.line
}

func (p *Parser) ReadListStart() {
	p.Expect("(")
	if !p.Valid() {
		return
	}

	p.listDepth++
	return
}

func (p *Parser) ReadListEnd() {
	p.Expect(")")
	if !p.Valid() {
		return
	}

	p.listDepth--
	return
}

func (p *Parser) ReadAtomList() []string {
	return p.ReadList(p.ReadAtom)
}

func (p *Parser) readStringList() []string {
	return p.ReadList(p.ReadString)
}

// NOTE: It's usually an error for the client to send a zero-item list.
// This method doesn't treat that as an error, but maybe should.
func (p *Parser) ReadList(tokenReader func() string) []string {
	p.ReadListStart()
	if !p.Valid() {
		return nil
	}

	list := make([]string, 0, 5)
	for {
		atom := tokenReader()
		if !p.Valid() {
			p.err = nil
			p.ReadListEnd()
			if !p.Valid() {
				return nil
			}
			break
		}

		list = append(list, atom)
		p.ReadSpace()
	}

	return list
}

func (p *Parser) ReadSequenceSet() *SequenceSet {
	set := &SequenceSet{}

	for {
		rng := p.readSequenceRange()
		if !p.Valid() {
			return set
		}
		set.Append(rng)

		if !p.accept(",") {
			break
		}
	}

	return set
}

func (p *Parser) readSequenceRange() SequenceRange {
	first := 0
	second := 0

	if p.accept("*") {
		first = Star
	} else {
		first = p.ReadInt()
		if first < 1 {
			p.err = ErrSeqNoOutOfBounds
		}
	}

	if !p.accept(":") {
		// single seq-number
		return [2]int{first, first}
	}

	if !p.Valid() {
		return SequenceRange{}
	}

	if p.accept("*") {
		second = Star
	} else {
		second = p.ReadInt()
		if second < 1 {
			p.err = ErrSeqNoOutOfBounds
		}
	}

	if !p.Valid() {
		return SequenceRange{}
	}

	return [2]int{first, second}
}

func (p *Parser) ReadFetchAttributes() []FetchAttribute {
	c := p.Peek()
	if !p.Valid() {
		return nil
	} else if c == '(' {
		return p.ReadFetchAttributeList()
	}

	macro := p.ReadAtom()
	if !p.Valid() {
		p.err = nil
		// Handle single-BODY[] case
		b := p.ReadBodyAttribute()
		if !p.Valid() {
			return nil
		} else {
			return []FetchAttribute{b}
		}
	}

	fields := []string{}
	switch macro {
	case "ALL":
		fields = []string{"FLAGS", "INTERNALDATE", "RFC822.SIZE", "ENVELOPE"}
	case "FAST":
		fields = []string{"FLAGS", "INTERNALDATE", "RFC822.SIZE"}
	case "FULL":
		fields = []string{"FLAGS", "INTERNALDATE", "RFC822.SIZE", "ENVELOPE", "BODY"}
	default:
		// Handle single-field case
		f, err := parseFetchAttribute(macro)
		if err != nil {
			p.err = err
			return nil
		} else {
			return []FetchAttribute{f}
		}
	}

	list := make([]FetchAttribute, 0, len(fields))
	for _, f := range fields {
		fa, err := parseFetchAttribute(f)
		if err != nil {
			p.err = err
			return nil
		}
		list = append(list, fa)
	}

	return list
}

// NOTE: It's usually an error for the client to send a zero-item list.
// This method doesn't treat that as an error, but maybe should.
func (p *Parser) ReadFetchAttributeList() []FetchAttribute {
	p.ReadListStart()
	if !p.Valid() {
		return nil
	}

	list := make([]FetchAttribute, 0, 5)
	fa := p.ReadFetchAttribute()
	list = append(list, fa)

	for p.accept(" ") {
		if !p.Valid() {
			return nil
		}

		fa := p.ReadFetchAttribute()
		list = append(list, fa)
	}

	p.ReadListEnd()
	return list
}

func (p *Parser) ReadFetchAttribute() FetchAttribute {
	field := p.ReadAtom()
	if !p.Valid() {
		p.err = nil
		return p.ReadBodyAttribute()
	}

	fa, err := parseFetchAttribute(field)
	if err != nil {
		p.err = err
	}
	return fa
}

func (p *Parser) ReadBodyAttribute() *BodyFetchAttribute {
	peek := p.readBodyAttributeStart()
	if !p.Valid() {
		return nil
	}

	fa := &BodyFetchAttribute{peek: peek}

	num := false
	for {
		c := p.Peek()
		if !p.Valid() {
			return nil
		}

		if c >= '1' && c <= '9' {
			i := p.ReadInt()
			if !p.Valid() {
				return nil
			}
			if len(fa.part) > 0 {
				fa.part += "."
			}
			fa.part += strconv.Itoa(i)
			num = true
		} else if c == '.' && num {
			num = false
		} else {
			break
		}
	}

	c := p.Peek()
	if !p.Valid() {
		return nil
	}

	if c == '.' {
		if fa.part == "" {
			// leading '.'
			p.err = ProtocolError("invalid part specifier")
			return nil
		}

		p.advance(1)
		if !p.Valid() {
			return nil
		}
	}

	c = p.Peek()
	if !p.Valid() {
		return nil
	}

	if c != ' ' && c != ']' {
		mode := p.ReadAtom()
		switch mode {
		case FetchText:
			fa.mode = FetchText
		case FetchHeader:
			fa.mode = FetchHeader
		case FetchHeaderFields:
			fa.mode = FetchHeaderFields
			p.Expect(" ")
			fa.headerList = p.readStringList()
			if !p.Valid() {
				return nil
			}
		case FetchHeaderFieldsNot:
			fa.mode = FetchHeaderFieldsNot
			fa.headerList = p.readStringList()
			if !p.Valid() {
				return nil
			}
		default:
			p.err = ProtocolError("invalid part specifier")
			return nil
		}
	}

	fa.hasPartial, fa.partial = p.readBodyAttributeEnd()
	return fa
}

func (p *Parser) readBodyAttributeStart() bool {
	p.Expect("BODY")
	peek := p.accept(".PEEK")
	p.Expect("[")

	p.inSection = true
	return peek
}

func (p *Parser) readBodyAttributeEnd() (hasPartial bool, rng [2]int) {
	p.Expect("]")
	if !p.Valid() {
		return
	}
	p.inSection = false

	c := p.Peek()
	if c == '<' {
		hasPartial = true
		rng = p.readBodyAttributePartial()
		return
	}

	return
}

func (p *Parser) readBodyAttributePartial() [2]int {
	p.Expect("<")
	first := p.ReadInt()
	p.Expect(".")
	second := p.ReadInt()
	p.Expect(">")

	return [2]int{first, second}
}

func (p *Parser) ReadFlagList() []Flag {
	strs := p.ReadAtomList()
	if !p.Valid() {
		return nil
	}

	list := make([]Flag, 0)
	for _, s := range strs {
		if isKnownFlag[s] {
			list = append(list, Flag(s))
		} else {
			p.err = ProtocolErrorf("unknown flag %q", s)
			return list
		}
	}

	return list
}

func (p *Parser) ReadSearch() (charset string, query Term) {
	charset = "us-ascii"
	p.ReadSpace()
	if p.accept("CHARSET") {
		p.ReadSpace()
		charset = p.ReadString()
		p.ReadSpace()
	}

	terms := []Term{p.ReadSearchKey()}
	for p.accept(" ") {
		terms = append(terms, p.ReadSearchKey())
	}

	if len(terms) == 1 {
		query = terms[0]
	} else {
		query = &BooleanTerm{
			Op:    OpAnd,
			Terms: terms,
		}
	}

	return charset, query
}

// Adapted from: https://code.google.com/p/go-imap/source/browse/go1/imap/reader.go
//
// normalize returns a normalized string copy of an atom. Non-flag atoms are
// converted to upper case. Flags are converted to title case (e.g. `\Seen`).
func normalize(atom string) string {
	norm := []byte(nil)
	want := byte(0) // Want upper case
	for i, c := range []byte(atom) {
		have := c & 0x20
		if c &= 0xDF; 'A' <= c && c <= 'Z' && have != want {
			norm = make([]byte, len(atom))
			break
		} else if i == 1 && atom[0] == '\\' {
			want = 0x20 // Want lower case starting at i == 2
		}
	}
	if norm == nil {
		return atom // Fast path: no changes
	}
	want = 0
	for i, c := range []byte(atom) {
		if c &= 0xDF; 'A' <= c && c <= 'Z' {
			norm[i] = c | want
		} else {
			norm[i] = atom[i]
		}
		if i == 1 && atom[0] == '\\' {
			want = 0x20
		}
	}
	return string(norm)
}
