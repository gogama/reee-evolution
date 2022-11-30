package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/mail"
	"os"
	"path/filepath"
	"time"

	"github.com/jhillyerd/enmime"

	"github.com/gogama/reee-evolution/daemon"
	_ "github.com/mattn/go-sqlite3"
)

type stmt int

type SQLite3Store struct {
	io.Closer
	db   *sql.DB
	stmt [numStmt]*sql.Stmt
}

func NewSQLite3(ctx context.Context, path string) (daemon.MessageStore, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	store := &SQLite3Store{db: db}

	err = store.init(ctx)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLite3Store) init(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	err = initSchema(ctx, conn)
	if err != nil {
		return err
	}
	err = s.prepareStmts(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (s *SQLite3Store) prepareStmts(ctx context.Context) error {
	for i := range s.stmt {
		var err error
		s.stmt[i], err = s.db.PrepareContext(ctx, stmtText[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLite3Store) Close() error {
	var firstErr error
	for i := range s.stmt {
		err := s.stmt[i].Close()
		if firstErr == nil {
			firstErr = err
		}
	}
	err := s.db.Close()
	if firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (s *SQLite3Store) GetMetadata(storeID string) (daemon.Metadata, bool, error) {
	sampledRows, err := s.stmt[getMetadataSampled].Query(storeID)
	if err != nil {
		return daemon.Metadata{}, false, err
	}
	defer func() {
		_ = sampledRows.Close()
	}()

	if !sampledRows.Next() {
		return daemon.Metadata{}, false, nil
	}

	var sampled bool
	err = sampledRows.Scan(&sampled)
	if err != nil {
		return daemon.Metadata{}, false, err
	}

	tagRows, err := s.stmt[getMetadataTags].Query(storeID)
	if err != nil {
		return daemon.Metadata{}, false, err
	}
	defer func() {
		_ = tagRows.Close()
	}()

	var tags map[string]string
	if tagRows.Next() {
		for {
			var k, v string
			err = tagRows.Scan(&k, &v)
			if err != nil {
				return daemon.Metadata{}, false, err
			}
			tags[k] = v
			if !tagRows.Next() {
				break
			}
		}
	}

	return daemon.NewMetadata(sampled, tags), false, nil
}

func (s *SQLite3Store) PutMessage(storeID string, msg *daemon.Message) error {
	insertTime := time.Now().Format(formatISO8601)
	isSampled := msg.IsSampled()

	var sendTime *string
	var fromAddress *string
	var fromAlias *string
	var toAddress *string
	var toAlias *string
	var toList *string
	var subject *string
	var ccAddress *string
	var ccAlias *string
	var ccList *string
	var senderAddress *string
	var senderAlias *string
	var inReplyToID *string
	var threadTopic *string
	var evolutionSource *string
	var mainHeaderJSON *[]byte
	var fullText *[]byte

	if date := msg.Envelope.GetHeader("Date"); date != "" {
		t, err := mail.ParseDate(date)
		if err != nil {
			return err
		}
		f := t.Format(formatISO8601)
		sendTime = &f
	}

	fromAddress, fromAlias, _ = parseAddressList(msg.Envelope, "From")
	toAddress, toAlias, toList = parseAddressList(msg.Envelope, "To")

	if value := msg.Envelope.GetHeader("Subject"); value != "" {
		subject = &value
	}

	ccAddress, ccAlias, ccList = parseAddressList(msg.Envelope, "CC")
	senderAddress, senderAlias, _ = parseAddressList(msg.Envelope, "Sender")
	inReplyToID, _, _ = parseAddressList(msg.Envelope, "In-Reply-To")
	threadTopic = optionalHeader(msg.Envelope, "Thread-Topic")
	evolutionSource = optionalHeader(msg.Envelope, "X-Evolution-Source")

	if isSampled {
		b, err := headersAsJSON(msg.Envelope)
		if err != nil {
			return err
		}
		mainHeaderJSON = &b
		fullText = &msg.FullText
	}

	_, err := s.stmt[putMessage].Exec(storeID, insertTime, isSampled, sendTime,
		fromAddress, fromAlias, toAddress, toAlias, toList,
		subject, ccAddress, ccAlias, ccList, senderAddress, senderAlias,
		inReplyToID, threadTopic, evolutionSource, mainHeaderJSON, fullText)

	return err
}

func (s *SQLite3Store) RecordEval(storeID string, r daemon.EvalRecord) error {
	return nil
}

func initSchema(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS message(
    id             		TEXT    PRIMARY KEY,
    insert_time        	TEXT 	NOT NULL,
    is_sampled   		INTEGER NOT NULL,
    send_time          	TEXT,
    from_address      	TEXT,
    from_alias        	TEXT,
    to_address        	TEXT,
    to_alias          	TEXT,
    to_list           	TEXT,
	subject           	TEXT,
    cc_address        	TEXT,
    cc_alias          	TEXT,
    cc_list           	TEXT,
    sender_address    	TEXT,
    sender_alias      	TEXT,
    in_reply_to_id    	TEXT,
    thread_topic       	TEXT,
    evolution_source	TEXT,
    main_header_json   	TEXT,
    full_text          	TEXT
);

CREATE TABLE IF NOT EXISTS message_tag(
    id 				INTEGER	PRIMARY KEY,
	message_id		TEXT 	NOT NULL,
	"key"       	TEXT 	NOT NULL,
	"value"     	TEXT,
	create_time 	TEXT 	NOT NULL,
	create_group 	TEXT 	NOT NULL,
	create_rule     TEXT    NOT NULL, 
	update_time 	TEXT    NOT NULL,
	update_group    TEXT    NOT NULL,
	update_rule     TEXT    NOT NULL,

	FOREIGN KEY(message_id) REFERENCES message(id) 
);

CREATE UNIQUE INDEX IF NOT EXISTS iu_message_tag_on_message_id_key
                 ON message_tag(message_id, "key");

CREATE TABLE IF NOT EXISTS message_eval(
    id 			    INTEGER	PRIMARY KEY,
    message_id      TEXT    NOT NULL,
	"group"    		TEXT    NOT NULL,
	rule        	TEXT    NOT NULL,
	eval_result 	TEXT    NOT NULL,                                       
	eval_start_time TEXT    NOT NULL,
	eval_end_time   TEXT	NOT NULL,
	eval_seconds    REAL    NOT NULL,

	FOREIGN KEY(message_id) REFERENCES message(id)
);

CREATE INDEX IF NOT EXISTS i_message_eval_on_message_id_id
          ON message_eval(message_id, id);

`)
	return err
}

const (
	getMetadataSampled stmt = iota
	getMetadataTags
	putMessage
	numStmt // TODO: Move this down to the end.
	recordEval

	formatISO8601 = "2006-01-02T15:04:05.999Z07:00"
)

var (
	stmtText = [numStmt]string{
		`SELECT is_sampled FROM message WHERE id = :id`,
		`SELECT "key", "value" FROM message_tag WHERE id = :id`,
		`INSERT INTO message(
                    id, insert_time, is_sampled, send_time,
                    from_address, from_alias, to_address, to_alias, to_list,
                    subject, cc_address, cc_alias, cc_list, sender_address, sender_alias,
                    in_reply_to_id, thread_topic, evolution_source, main_header_json, full_text)
			VALUES (
			        :id, :insert_time, :is_sampled, :send_time,
			        :from_address, :from_alias, :to_address, :to_alias, :to_list,
			        :subject, :cc_address, :cc_alias, :cc_list, :sender_address, :sender_alias,
			        :in_reply_to_id, :thread_topic, :evolution_source, :main_header_json, :full_text)`,
	}
)

func parseAddressList(e *enmime.Envelope, hdr string) (address, alias, list *string) {
	value := e.GetHeader(hdr)
	addrs, err := mail.ParseAddressList(value)
	if err != nil {
		return
	}
	if len(addrs) == 1 {
		address = &addrs[0].Address
		alias = &addrs[0].Name
	} else {
		list = &value
	}
	return
}

func optionalHeader(e *enmime.Envelope, hdr string) *string {
	if value := e.GetHeader(hdr); value != "" {
		return &value
	}
	return nil
}

func headersAsJSON(e *enmime.Envelope) ([]byte, error) {
	keys := e.GetHeaderKeys()
	hdrs := make(map[string]string, len(keys))
	for _, key := range keys {
		hdrs[key] = e.GetHeader(key)
	}
	return json.Marshal(&hdrs)
}
