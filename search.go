package imap

import (
	"time"
)

type TermVisitor interface {
	VisitAllTerm(*AllTerm)
	VisitBooleanTerm(*BooleanTerm)
	VisitDateTerm(*DateTerm)
	VisitFlagTerm(*FlagTerm)
	VisitIntTerm(*IntTerm)
	VisitSetTerm(*SetTerm)
	VisitStringTerm(*StringTerm)
	VisitUnaryTerm(*UnaryTerm)
}

type Term interface {
	Accept(TermVisitor)
}

type AllTerm struct{}

func (t *AllTerm) Accept(v TermVisitor) { v.VisitAllTerm(t) }

type BooleanTerm struct {
	Op    Op
	Terms []Term
}

func (t *BooleanTerm) Accept(v TermVisitor) { v.VisitBooleanTerm(t) }

type UnaryTerm struct {
	Op   Op
	Term Term
}

func (t *UnaryTerm) Accept(v TermVisitor) { v.VisitUnaryTerm(t) }

type FlagTerm struct {
	Flag    Flag
	Present bool
}

func (t *FlagTerm) Accept(v TermVisitor) { v.VisitFlagTerm(t) }

type StringTerm struct {
	Op     Op
	Field  Field
	String string
}

func (t *StringTerm) Accept(v TermVisitor) { v.VisitStringTerm(t) }

type IntTerm struct {
	Op    Op
	Field Field
	Int   int
}

func (t *IntTerm) Accept(v TermVisitor) { v.VisitIntTerm(t) }

type SetTerm struct {
	Op    Op
	Field Field
	Set   *SequenceSet
}

func (t *SetTerm) Accept(v TermVisitor) { v.VisitSetTerm(t) }

type DateTerm struct {
	Op    Op
	Field Field
	Date  time.Time
}

func (t *DateTerm) Accept(v TermVisitor) { v.VisitDateTerm(t) }

type Op int

const (
	OpAnd Op = iota
	OpOr
	OpNot

	OpEquals
	OpLT
	OpGTE

	// Strings
	OpContains

	// Sets
	OpIn
)

type Field string

const (
	FromField Field = "From"
	ToField         = "To"
	CcField         = "Cc"
	BccField        = "Bcc"

	SubjectField = "Subject"
	TextField    = "Text"
	BodyField    = "Body"

	MSNField          = "MSN"
	UIDField          = "UID"
	SizeField         = "Size"
	DateField         = "Date"
	InternalDateField = "InternalDate"
)

// Deviates from RFC3501 somewhat:
// * BODY and TEXT both perform general text searches
// * Handling of "ALL" is not super strict
func (p *Parser) ReadSearchKey() Term {
	if !p.Valid() {
		return nil
	}

	if p.accept("all") {
		return &AllTerm{}
	} else if p.accept("answered") {
		return &FlagTerm{Flag: FlagAnswered, Present: true}
	} else if p.accept("bcc") {
		p.ReadSpace()
		s := p.ReadAstring()
		return &StringTerm{
			Op:     OpContains,
			Field:  BccField,
			String: s,
		}
	} else if p.accept("before") {
		p.ReadSpace()
		dt := p.ReadDate()
		return &DateTerm{
			Op:    OpLT,
			Field: InternalDateField,
			Date:  dt,
		}
	} else if p.accept("body") {
		p.ReadSpace()
		s := p.ReadString()
		return &StringTerm{
			Op:     OpContains,
			Field:  BodyField,
			String: s,
		}
	} else if p.accept("cc") {
		p.ReadSpace()
		s := p.ReadAstring()
		return &StringTerm{
			Op:     OpContains,
			Field:  CcField,
			String: s,
		}
	} else if p.accept("deleted") {
		return &FlagTerm{Flag: FlagDeleted, Present: true}
	} else if p.accept("flagged") {
		return &FlagTerm{Flag: FlagFlagged, Present: true}
	} else if p.accept("from") {
		p.ReadSpace()
		s := p.ReadAstring()
		return &StringTerm{
			Op:     OpContains,
			Field:  FromField,
			String: s,
		}
	} else if p.accept("keyword") {
		p.ReadSpace()
		a := p.ReadAtom()
		return &FlagTerm{
			Flag:    Flag(a),
			Present: true,
		}
	} else if p.accept("new") {
		return &BooleanTerm{
			Op: OpAnd,
			Terms: []Term{
				&FlagTerm{Flag: FlagRecent, Present: true},
				&FlagTerm{Flag: FlagSeen, Present: false},
			},
		}
	} else if p.accept("old") {
		return &FlagTerm{Flag: FlagRecent, Present: false}
	} else if p.accept("on") {
		p.ReadSpace()
		dt := p.ReadDate()
		return &DateTerm{
			Op:    OpEquals,
			Field: InternalDateField,
			Date:  dt,
		}
	} else if p.accept("recent") {
		return &FlagTerm{Flag: FlagRecent, Present: true}
	} else if p.accept("seen") {
		return &FlagTerm{Flag: FlagSeen, Present: true}
	} else if p.accept("since") {
		p.ReadSpace()
		dt := p.ReadDate()
		return &DateTerm{
			Op:    OpGTE,
			Field: InternalDateField,
			Date:  dt,
		}
	} else if p.accept("subject") {
		p.ReadSpace()
		s := p.ReadAstring()
		return &StringTerm{
			Op:     OpContains,
			Field:  SubjectField,
			String: s,
		}
	} else if p.accept("text") {
		p.ReadSpace()
		s := p.ReadAstring()
		return &StringTerm{
			Op:     OpContains,
			Field:  TextField,
			String: s,
		}
	} else if p.accept("to") {
		p.ReadSpace()
		s := p.ReadAstring()
		return &StringTerm{
			Op:     OpContains,
			Field:  ToField,
			String: s,
		}
	} else if p.accept("unanswered") {
		return &FlagTerm{Flag: FlagAnswered, Present: false}
	} else if p.accept("undeleted") {
		return &FlagTerm{Flag: FlagDeleted, Present: false}
	} else if p.accept("unflagged") {
		return &FlagTerm{Flag: FlagFlagged, Present: false}
	} else if p.accept("unkeyword") {
		p.ReadSpace()
		a := p.ReadAtom()
		return &FlagTerm{Flag: Flag(a), Present: false}
	} else if p.accept("unseen") {
		return &FlagTerm{Flag: FlagSeen, Present: false}
	} else if p.accept("draft") {
		return &FlagTerm{Flag: FlagDraft, Present: true}
	} else if p.accept("header") {
		p.ReadSpace()
		field := p.ReadAstring()
		p.ReadSpace()
		s := p.ReadString()
		return &StringTerm{
			Op:     OpContains,
			Field:  Field("header." + field),
			String: s,
		}
	} else if p.accept("larger") {
		p.ReadSpace()
		s := p.ReadInt()
		return &IntTerm{
			Op:    OpGTE,
			Field: SizeField,
			Int:   s,
		}
	} else if p.accept("not") {
		p.ReadSpace()
		term := p.ReadSearchKey()
		return &UnaryTerm{
			Op:   OpNot,
			Term: term,
		}
	} else if p.accept("or") {
		p.ReadSpace()
		key1 := p.ReadSearchKey()
		p.ReadSpace()
		key2 := p.ReadSearchKey()
		if _, ok := key1.(*AllTerm); ok {
			return key2
		} else if _, ok := key2.(*AllTerm); ok {
			return key1
		}
		return &BooleanTerm{
			Op:    OpOr,
			Terms: []Term{key1, key2},
		}
	} else if p.accept("sentbefore") {
		p.ReadSpace()
		dt := p.ReadDate()
		return &DateTerm{
			Op:    OpLT,
			Field: DateField,
			Date:  dt,
		}
	} else if p.accept("senton") {
		p.ReadSpace()
		dt := p.ReadDate()
		return &DateTerm{
			Op:    OpEquals,
			Field: DateField,
			Date:  dt,
		}
	} else if p.accept("sentsince") {
		p.ReadSpace()
		dt := p.ReadDate()
		return &DateTerm{
			Op:    OpGTE,
			Field: DateField,
			Date:  dt,
		}
	} else if p.accept("smaller") {
		p.ReadSpace()
		s := p.ReadInt()
		return &IntTerm{
			Op:    OpLT,
			Field: SizeField,
			Int:   s,
		}
	} else if p.accept("uid") {
		p.ReadSpace()
		set := p.ReadSequenceSet()
		return &SetTerm{
			Op:    OpIn,
			Field: UIDField,
			Set:   set,
		}
	} else if p.accept("undraft") {
		return &FlagTerm{Flag: FlagDraft, Present: false}
	} else if p.accept("(") {
		term := &BooleanTerm{
			Op:    OpAnd,
			Terms: []Term{p.ReadSearchKey()},
		}
		for !p.accept(")") {
			p.ReadSpace()
			term.Terms = append(term.Terms, p.ReadSearchKey())
			if !p.Valid() {
				break
			}
		}

		if len(term.Terms) == 1 {
			return term.Terms[0]
		}

		return term
	} else {
		// sequence-set
		set := p.ReadSequenceSet()
		if !p.Valid() {
			return nil
		}
		return &SetTerm{
			Op:    OpIn,
			Field: MSNField,
			Set:   set,
		}
	}

	p.err = ProtocolError("invalid search key")
	return nil
}
