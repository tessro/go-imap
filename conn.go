package imap

import (
	"fmt"
	"io"
)

type Writer interface {
	io.Writer

	Splat(msg string)
	Continuation(msg string)
	Ok(r *Request)
	OkWithCode(r *Request, responseCode string)
	No(r *Request, err error)
	Bad(r *Request, err error)
}

type Reader interface {
	io.Reader
}

type Conn struct {
	rwc    io.ReadWriteCloser
	parser *Parser
}

func NewConn(rwc io.ReadWriteCloser) *Conn {
	conn := &Conn{
		rwc: rwc,
	}
	conn.parser = NewParser(conn)

	return conn
}

func (c *Conn) Write(b []byte) (int, error) {
	return c.rwc.Write(b)
}

func (c *Conn) Close() error {
	return c.rwc.Close()
}

func (c *Conn) Splat(msg string) {
	fmt.Fprintf(c, "* %s\r\n", msg)
}

func (c *Conn) Continuation(msg string) {
	fmt.Fprintf(c, "+ %s\r\n", msg)
}

func (c *Conn) Ok(r *Request) {
	fmt.Fprintf(c, "%s OK %s completed\r\n", r.Tag, r.Command)
}

func (c *Conn) OkWithCode(r *Request, responseCode string) {
	fmt.Fprintf(c, "%s OK [%s] %s completed\r\n", r.Tag, responseCode, r.Command)
}

func (c *Conn) No(r *Request, err error) {
	tag := "*"
	if r != nil {
		tag = r.Tag
	}

	fmt.Fprintf(c, "%s NO %s\r\n", tag, err)
}

func (c *Conn) Bad(r *Request, err error) {
	tag := "*"
	if r != nil {
		tag = r.Tag
	}

	fmt.Fprintf(c, "%s BAD %s\r\n", tag, err)
}

func (c *Conn) DiscardLine() {
	c.parser.DiscardLine()
}
