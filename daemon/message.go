package daemon

import (
	"time"

	"github.com/jhillyerd/enmime"
)

type Metadata struct {
	sampled bool
	tags    map[string]string
	// TODO: Will need a thing to track changes.
}

func NewMetadata(sampled bool, tags map[string]string) Metadata {
	copiedTags := make(map[string]string)
	for k, v := range tags {
		copiedTags[k] = v
	}
	return Metadata{
		sampled: sampled,
		tags:    copiedTags,
	}
}

type Message struct {
	Envelope *enmime.Envelope
	metadata Metadata
}

type MessageCache interface {
	Get(cacheKey string) *Message
	Put(cacheKey string, msg *Message, size uint64)
}

type MessageStore interface {
	GetMetadata(storeID string) (Metadata, bool, error)
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
		// TODO: Parse away any angle brackets.
		return "Message-ID:" + id
	} else {
		return "MD5-Sum:" + md5Sum
	}
}
