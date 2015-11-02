package imap

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/paulrosania/go-mail"
)

type FetchAttribute interface {
	Name() string
	Marshal(*Message) string
}

func parseFetchAttribute(field string) (FetchAttribute, error) {
	switch field {
	case "BODY":
		return BodystructureFetchAttribute(false), nil
	case "BODYSTRUCTURE":
		return BodystructureFetchAttribute(true), nil
	case "FLAGS":
		return FlagsFetchAttribute, nil
	case "INTERNALDATE":
		return InternalDateFetchAttribute, nil
	case "RFC822":
		return RFC822FetchAttribute, nil
	case "RFC822.HEADER":
		return RFC822HeaderFetchAttribute, nil
	case "RFC822.SIZE":
		return RFC822SizeFetchAttribute, nil
	case "RFC822.TEXT":
		return RFC822TextFetchAttribute, nil
	case "ENVELOPE":
		return EnvelopeFetchAttribute, nil
	case "UID":
		return UIDFetchAttribute, nil
	}

	return nil, ProtocolErrorf("unknown field %q", field)
}

type bodyFetchMode string

const (
	FetchAll             bodyFetchMode = ""
	FetchText                          = "TEXT"
	FetchHeader                        = "HEADER"
	FetchHeaderFields                  = "HEADER.FIELDS"
	FetchHeaderFieldsNot               = "HEADER.FIELDS.NOT"
)

type BodyFetchAttribute struct {
	peek       bool
	part       string
	mode       bodyFetchMode
	headerList []string
	hasPartial bool
	partial    [2]int
}

func (fa BodyFetchAttribute) Name() string {
	partialPart := ""
	if fa.hasPartial {
		partialPart = fmt.Sprintf("<%d>", fa.partial[0])
	}

	sectionPart := fa.part
	if fa.mode != FetchAll {
		if sectionPart != "" {
			sectionPart += "."
		}
		sectionPart += string(fa.mode)
	}
	if len(fa.headerList) > 0 {
		first := true
		sectionPart += " ("
		for _, h := range fa.headerList {
			if !first {
				sectionPart += " "
			}
			sectionPart += quoteString(h)
			first = false
		}
		sectionPart += ")"
	}

	modePart := fa.mode
	if len(modePart) > 0 {
		modePart = "." + modePart
	}

	return fmt.Sprintf("BODY[%s]%s", sectionPart, partialPart)
}

func (fa BodyFetchAttribute) Marshal(msg *Message) string {
	result := ""
	if fa.part == "" {
		switch fa.mode {
		case FetchText:
			result = msg.AsText(true)
		case FetchHeader:
			result = msg.Header.AsText(true)
		case FetchHeaderFields:
			keep := make(map[string]bool)
			for _, name := range fa.headerList {
				keep[strings.ToLower(name)] = true
			}
			i := 0
			for i < len(msg.Header.Fields) {
				f := msg.Header.Fields[i]
				if !keep[strings.ToLower(f.Name())] {
					msg.Header.Remove(f)
				} else {
					i++
				}
			}
			result = msg.Header.AsText(true)
		case FetchHeaderFieldsNot:
			for _, name := range fa.headerList {
				msg.Header.RemoveAllNamed(name)
			}
			result = msg.Header.AsText(true)
		default:
			result = msg.RFC822(true)
		}
	} else {
		part := msg.BodyPart(fa.part, false)
		if part == nil {
			// TODO: handle missing part
		} else {
			switch fa.mode {
			case FetchText:
				result = part.AsText(true)
			case FetchHeader:
				result = part.Header.AsText(true)
			case FetchHeaderFields:
				keep := make(map[string]bool)
				for _, name := range fa.headerList {
					keep[strings.ToLower(name)] = true
				}
				i := 0
				for i < len(part.Header.Fields) {
					f := part.Header.Fields[i]
					if !keep[strings.ToLower(f.Name())] {
						part.Header.RemoveAllNamed(f.Name())
					} else {
						i++
					}
				}
				result = part.Header.AsText(true)
			case FetchHeaderFieldsNot:
				for _, name := range fa.headerList {
					part.Header.RemoveAllNamed(name)
				}
				result = part.Header.AsText(true)
			default:
				buf := bytes.NewBuffer(make([]byte, 0, 10000))
				buf.WriteString(part.Header.AsText(true))
				buf.WriteString("\n")
				buf.WriteString(part.AsText(true))
				result = buf.String()
			}
		}
	}

	if fa.hasPartial {
		l := fa.partial[0]
		if l > len(result) {
			l = len(result)
		}
		r := fa.partial[1]
		if r < l {
			r = l
		}
		result = result[l:r]
	}

	return fmt.Sprintf("{%d}\r\n%s", len([]byte(result)), result)
}

type BodystructureFetchAttribute bool

func (includeExtensions BodystructureFetchAttribute) Name() string {
	if includeExtensions {
		return "BODYSTRUCTURE"
	} else {
		return "BODY"
	}
}

func (includeExtensions BodystructureFetchAttribute) marshalPart(p *mail.Part) string {
	ct := p.Header.ContentType()
	buf := bytes.NewBuffer(make([]byte, 0, 512))
	buf.WriteRune('(')
	if len(p.Parts) == 0 {
		contentType := "text"
		subtype := "plain"
		if ct != nil {
			contentType = ct.Type
			subtype = ct.Subtype
		}

		buf.WriteString(contentType)

		buf.WriteRune(' ')
		buf.WriteString(subtype)

		// params
		// TODO
		buf.WriteString(" (")
		buf.WriteRune(')')

		// id
		// TODO
		buf.WriteRune(' ')
		buf.WriteString("NIL")

		// desc
		buf.WriteRune(' ')
		contentDescription := p.Header.ContentDescription()
		if contentDescription == "" {
			buf.WriteString("NIL")
		} else {
			buf.WriteString(contentDescription)
		}

		// enc
		// TODO
		buf.WriteRune(' ')
		buf.WriteString(`"8-BIT"`)

		// octets
		octets := len([]byte(p.Text))
		buf.WriteRune(' ')
		buf.WriteString(strconv.Itoa(octets))

		if contentType == "text" {
			// lines
			// TODO
			lines := strings.Count(p.Text, "\n")
			buf.WriteRune(' ')
			buf.WriteString(strconv.Itoa(lines))
		} else if contentType == "message" {
			// envelope
			buf.WriteRune(' ')
			buf.WriteString("")

			// body
			buf.WriteRune(' ')
			buf.WriteString("")

			// lines
			lines := strings.Count(p.Text, "\n")
			buf.WriteRune(' ')
			buf.WriteString(strconv.Itoa(lines))
		}

		if includeExtensions {
			// TODO
			buf.WriteRune(' ')
			buf.WriteString("NIL")
		}
	} else {
		for _, pt := range p.Parts {
			buf.WriteString(includeExtensions.marshalPart(pt))
		}

		if includeExtensions {
			// TODO
			buf.WriteRune(' ')
			buf.WriteString("NIL")
		}
	}
	buf.WriteRune(')')
	return buf.String()
}

func (includeExtensions BodystructureFetchAttribute) Marshal(msg *Message) string {
	buf := bytes.NewBuffer(make([]byte, 0, 512))
	if len(msg.Parts) == 0 {
		// should never happen
	} else if len(msg.Parts) == 1 {
		buf.WriteString(includeExtensions.marshalPart(msg.Parts[0]))
	} else {
		buf.WriteRune('(')
		for _, p := range msg.Parts {
			buf.WriteString(includeExtensions.marshalPart(p))
		}

		ct := msg.Header.ContentType()
		buf.WriteRune(' ')
		buf.WriteString(ct.Subtype)

		if includeExtensions {
			buf.WriteRune(' ')
			// TODO
			buf.WriteString("NIL")
		}

		buf.WriteRune(')')
	}
	return buf.String()
}

type BasicFetchAttribute struct {
	name      string
	marshaler func(*Message) string
}

func (fa BasicFetchAttribute) Name() string {
	return fa.name
}

func (fa BasicFetchAttribute) Marshal(m *Message) string {
	return fa.marshaler(m)
}

var (
	EnvelopeFetchAttribute     = BasicFetchAttribute{"ENVELOPE", MarshalEnvelope}
	FlagsFetchAttribute        = BasicFetchAttribute{"FLAGS", MarshalFlags}
	InternalDateFetchAttribute = BasicFetchAttribute{"INTERNALDATE", MarshalInternalDate}
	RFC822FetchAttribute       = BasicFetchAttribute{"RFC822", MarshalRFC822}
	RFC822HeaderFetchAttribute = BasicFetchAttribute{"RFC822.HEADER", MarshalRFC822Header}
	RFC822SizeFetchAttribute   = BasicFetchAttribute{"RFC822.SIZE", MarshalRFC822Size}
	RFC822TextFetchAttribute   = BasicFetchAttribute{"RFC822.TEXT", MarshalRFC822Text}
	UIDFetchAttribute          = BasicFetchAttribute{"UID", MarshalUID}
)

func MarshalEnvelope(m *Message) string {
	return headerToEnvelopeString(m.Header)
}

func MarshalFlags(m *Message) string {
	flags := []string{}
	for _, f := range m.Flags {
		flags = append(flags, f.String())
	}

	return fmt.Sprintf("(%s)", strings.Join(flags, " "))
}

func MarshalInternalDate(m *Message) string {
	return quoteString(m.ReceivedAt.Format(time.RFC822))
}

func MarshalRFC822(m *Message) string {
	return quoteString(m.RFC822(true))
}

func MarshalRFC822Header(m *Message) string {
	return quoteString(m.Header.AsText(true))
}

func MarshalRFC822Size(m *Message) string {
	return strconv.Itoa(m.RFC822Size)
}

func MarshalRFC822Text(m *Message) string {
	return quoteString(m.Body(true))
}

func MarshalUID(m *Message) string {
	return strconv.Itoa(m.UID)
}
