package imap

import (
	"strings"
)

type Request struct {
	*Parser
	Tag     string
	Command string
}

func (c *Conn) ReadRequest() (*Request, error) {
	p := c.parser
	req := &Request{Parser: p}

	tag := req.ReadAtom()
	req.ReadSpace()
	cmd := req.ReadAtom()

	if !p.Valid() {
		return nil, p.Err()
	}

	req.Tag = tag
	req.Command = strings.ToUpper(cmd)

	return req, nil
}
