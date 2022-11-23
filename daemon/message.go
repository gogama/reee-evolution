package daemon

import (
	"time"

	"github.com/jhillyerd/enmime"
)

type Metadata struct {
	tags    map[string]string
	sampled bool
}

type Message struct {
	Envelope *enmime.Envelope
	metadata Metadata
}

type MessageCache interface {
	Get(cacheKey string) *Message
	Put(cacheKey string, msg *Message, size int64)
}

type MessageStore interface {
	GetMetadata(storeID string) (Metadata, bool)
	PutMessage(storeID string, msg *Message) error
	RecordEval(storeID string, r EvalRecord) error
}

type EvalRecord struct {
	Message   *Message
	Hash      string
	StartTime time.Time
	Group     string
	Rules     []RuleEvalRecord
}

type RuleEvalRecord struct {
	StartTime time.Time
	Rule      string
	Result    int // TODO: Should be a meaningful value.
}

func toStoreID(e *enmime.Envelope, md5Sum string) string {
	id := e.GetHeader("Message-ID")
	if id != "" {
		return "Message-ID:" + id
	} else {
		return "MD5-Sum:" + md5Sum
	}
}
