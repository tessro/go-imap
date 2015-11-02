package imap

import (
	"fmt"
	"strings"

	"github.com/paulrosania/go-mail"
)

func quoteString(s string) string {
	return fmt.Sprintf("%q", s)
}

func headerToImapString(h *mail.Header, key string) string {
	for _, fld := range h.Fields {
		if fld.Name() == key {
			return fmt.Sprintf("%q", fld.Value)
		}
	}
	return "NIL"
}

func headerToEnvelopeString(h *mail.Header) string {
	return fmt.Sprintf("(%s %s %s %s %s %s %s %s %s %s)",
		headerToImapString(h, "Date"),
		headerToImapString(h, "Subject"),
		addressHeaderToStructureList(h.Get("From")),
		addressHeaderToStructureList(h.Get("Sender")),
		addressHeaderToStructureList(h.Get("Reply-To")),
		addressHeaderToStructureList(h.Get("To")),
		addressHeaderToStructureList(h.Get("Cc")),
		addressHeaderToStructureList(h.Get("Bcc")),
		headerToImapString(h, "In-Reply-To"),
		headerToImapString(h, "Message-Id"))
}

func addressHeaderToStructureList(header string) string {
	if header == "" {
		return "NIL"
	}

	ap := mail.NewAddressParser(header)
	addrs := make([]string, 0, len(ap.Addresses))
	for _, a := range ap.Addresses {
		name := a.Name(false)
		if name == "" {
			name = "NIL"
		} else {
			name = quoteString(name)
		}
		str := fmt.Sprintf("(%s NIL %q %q)", name, a.Localpart, a.Domain)
		addrs = append(addrs, str)
	}

	return fmt.Sprintf("(%s)", strings.Join(addrs, " "))
}
