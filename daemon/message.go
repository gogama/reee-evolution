package daemon

import (
	"net/mail"
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

func (m *Metadata) IsSampled() bool {
	return m.sampled
}

type Message struct {
	Envelope *enmime.Envelope
	FullText []byte
	metadata Metadata
}

func (m *Message) IsSampled() bool {
	return m.metadata.IsSampled()
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
	// TODO: Add tags here.
}

func toStoreID(e *enmime.Envelope, md5Sum string) string {
	id := e.GetHeader("Message-ID")
	if id != "" {
		addr, err := mail.ParseAddress(id)
		if err == nil {
			return "Message-ID:" + addr.Address
		}
	}
	return "MD5-Sum:" + md5Sum
}
