package imap

import (
	"time"

	"github.com/paulrosania/go-mail"
)

type Flag string

const (
	FlagAnswered Flag = `\Answered`
	FlagFlagged       = `\Flagged`
	FlagDeleted       = `\Trashed`
	FlagSeen          = `\Seen`
	FlagDraft         = `\Draft`
	FlagRecent        = `\Recent`

	KeywordMDNSent       Flag = `$MDNSent`
	KeywordForwarded          = `$Forwarded`
	KeywordSubmitPending      = `$SubmitPending`
	KeywordSubmitted          = `$Submitted`
)

var KnownFlags = []Flag{
	FlagAnswered,
	FlagFlagged,
	FlagDeleted,
	FlagSeen,
	FlagDraft,
	FlagRecent,

	KeywordMDNSent,
	KeywordForwarded,
	KeywordSubmitPending,
	KeywordSubmitted,
}

var isKnownFlag = map[string]bool{}

func init() {
	for _, f := range KnownFlags {
		isKnownFlag[f.String()] = true
	}
}

type Message struct {
	mail.Message

	UID        int
	Flags      []Flag
	ReceivedAt time.Time
}

func (f Flag) String() string {
	return string(f)
}
