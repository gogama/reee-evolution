package daemon

import (
	"net/mail"
	"sync"
	"time"

	"github.com/jhillyerd/enmime"
)

type Metadata struct {
	sampled bool
	tags    map[string]string
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
	fullText []byte
	metadata Metadata
	lock     sync.RWMutex
}

func (m *Message) FullText() []byte {
	return m.fullText
}

func (m *Message) IsSampled() bool {
	return m.metadata.sampled
}

type MessageCache interface {
	Get(cacheKey string) *Message
	Put(cacheKey string, msg *Message, size uint64)
}

type MessageStore interface {
	GetMetadata(storeID string) (Metadata, bool, error)
	PutMessage(storeID string, msg *Message) error
	RecordEval(storeID string, r *EvalRecord) error
}

type EvalRecord struct {
	Message   *Message
	group     string
	startTime time.Time
	endTime   time.Time
	rules     []*RuleEvalRecord
}

func (rec *EvalRecord) Group() string {
	return rec.group
}

func (rec *EvalRecord) StartTime() time.Time {
	return rec.startTime
}

func (rec *EvalRecord) EndTime() time.Time {
	return rec.endTime
}

func (rec *EvalRecord) RuleLen() int {
	return len(rec.rules)
}

func (rec *EvalRecord) Rule(i int) *RuleEvalRecord {
	return rec.rules[i]
}

type RuleEvalRecord struct {
	evalRecord *EvalRecord
	rule       string
	startTime  time.Time
	endTime    time.Time
	match      bool
	err        error
	tagChanges []TagChange
}

func (rec *RuleEvalRecord) Rule() string {
	return rec.rule
}

func (rec *RuleEvalRecord) StartTime() time.Time {
	return rec.startTime
}

func (rec *RuleEvalRecord) EndTime() time.Time {
	return rec.endTime
}

func (rec *RuleEvalRecord) Match() bool {
	return rec.match
}

func (rec *RuleEvalRecord) Err() error {
	return rec.err
}

func (rec *RuleEvalRecord) Keys() []string {
	msg := rec.evalRecord.Message
	lock := &msg.lock
	lock.RLock()
	defer lock.RUnlock()
	keys := make([]string, 0, len(msg.metadata.tags))
	for key := range msg.metadata.tags {
		keys = append(keys, key)
	}
	return keys
}

func (rec *RuleEvalRecord) GetTag(key string) (value string, hit bool) {
	msg := rec.evalRecord.Message
	lock := &msg.lock
	lock.RLock()
	defer lock.RUnlock()
	value, hit = msg.metadata.tags[key]
	return
}

func (rec *RuleEvalRecord) SetTag(key, value string) {
	msg := rec.evalRecord.Message
	lock := &msg.lock
	lock.Lock()
	defer lock.Unlock()
	if msg.metadata.tags == nil {
		msg.metadata.tags = make(map[string]string)
	}
	msg.metadata.tags[key] = value
	rec.tagChanges = append(rec.tagChanges, TagChange{time.Now(), key, &value})
}

func (rec *RuleEvalRecord) DeleteTag(key string) {
	msg := rec.evalRecord.Message
	lock := &msg.lock
	lock.Lock()
	defer lock.Unlock()
	delete(msg.metadata.tags, key)
	rec.tagChanges = append(rec.tagChanges, TagChange{time.Now(), key, nil})
}

func (rec *RuleEvalRecord) TagChangeLen() int {
	return len(rec.tagChanges)
}

func (rec *RuleEvalRecord) TagChange(i int) TagChange {
	return rec.tagChanges[i]
}

type TagChange struct {
	Time  time.Time
	Key   string
	Value *string
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
