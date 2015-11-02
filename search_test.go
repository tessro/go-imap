package imap_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/paulrosania/go-imap"
	"github.com/stretchr/testify/assert"
)

type Buffer struct {
	io.ReadWriter
}

func (b *Buffer) Close() error {
	return nil
}

func NewBuffer(input string) *Buffer {
	// Parser always reads lines, so make sure to give it one
	return &Buffer{bytes.NewBufferString(input + "\r\n")}
}

func NewParser(input string) *imap.Parser {
	rwc := NewBuffer(input)
	conn := imap.NewConn(rwc)
	parser := imap.NewParser(conn)
	return parser
}

func TestReadSearch(t *testing.T) {
	cs, actual := NewParser(` FLAGGED SINCE 1-Feb-1994 NOT FROM "Smith"`).ReadSearch()

	expected := &imap.BooleanTerm{
		Op: imap.OpAnd,
		Terms: []imap.Term{
			&imap.FlagTerm{Flag: imap.FlagFlagged, Present: true},
			&imap.DateTerm{
				Op:    imap.OpGTE,
				Field: imap.InternalDateField,
				Date:  time.Date(1994, time.February, 1, 0, 0, 0, 0, time.UTC),
			},
			&imap.UnaryTerm{
				Op: imap.OpNot,
				Term: &imap.StringTerm{
					Op:     imap.OpContains,
					Field:  imap.FromField,
					String: "Smith",
				},
			},
		},
	}

	assert.Equal(t, "us-ascii", cs, "charset should default to US-ASCII")
	assert.Equal(t, expected, actual, "parsed search query incorrectly")
}

func TestSearchParsesCharset(t *testing.T) {
	cs, actual := NewParser(` CHARSET UTF-8 TEXT XXXXXX`).ReadSearch()

	expected := &imap.StringTerm{
		Op:     imap.OpContains,
		Field:  imap.TextField,
		String: "XXXXXX",
	}

	assert.Equal(t, "UTF-8", cs, "charset should be UTF-8")
	assert.Equal(t, expected, actual)
}
