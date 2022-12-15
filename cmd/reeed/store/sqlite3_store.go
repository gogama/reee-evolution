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
		tags = make(map[string]string)
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

	return daemon.NewMetadata(sampled, tags), true, nil
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
		b2 := msg.FullText()
		fullText = &b2
	}

	_, err := s.stmt[putMessage].Exec(storeID, insertTime, isSampled, sendTime,
		fromAddress, fromAlias, toAddress, toAlias, toList,
		subject, ccAddress, ccAlias, ccList, senderAddress, senderAlias,
		inReplyToID, threadTopic, evolutionSource, mainHeaderJSON, fullText)

	return err
}

func (s *SQLite3Store) RecordEval(storeID string, r *daemon.EvalRecord) error {
	// Set up a transaction.
	var tx *sql.Tx
	var err error
	tx, err = s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// Get the final result of the group evaluation if there was one.
	var stop *bool
	var errStr *string
	m := r.RuleLen()
	if m > 0 {
		rr := r.Rule(m - 1)
		boolValue := rr.Stop()
		stop = &boolValue
		if lastRuleErr := rr.Err(); lastRuleErr != nil {
			stop = nil
			strValue := lastRuleErr.Error()
			errStr = &strValue
		}
	}

	// Insert the root group evaluation record and get back its ID.
	var result sql.Result
	result, err = s.stmt[putGroupEvalRecord].Exec(storeID, r.Group(),
		r.StartTime().Format(formatISO8601), r.EndTime().Format(formatISO8601), r.EndTime().Sub(r.StartTime()).Seconds(),
		stop, errStr)
	if err != nil {
		return err
	}
	groupEvalID, err := result.LastInsertId()
	if err != nil {
		return err
	}

	// Insert each rule evaluation record, alongside the tag changes for
	// that rule.
	for i := 0; i < m; i++ {
		rr := r.Rule(i)
		if ruleErr := rr.Err(); err == nil {
			boolValue := rr.Stop()
			stop = &boolValue
			errStr = nil
		} else {
			stop = nil
			strValue := ruleErr.Error()
			errStr = &strValue
		}
		_, err = s.stmt[putRuleEvalRecord].Exec(groupEvalID, rr.Rule(),
			rr.StartTime().Format(formatISO8601), rr.EndTime().Format(formatISO8601), rr.EndTime().Sub(rr.StartTime()).Seconds(),
			stop, errStr)
		if err != nil {
			return err
		}

		n := rr.TagChangeLen()
		for j := 0; j < n; j++ {
			tc := rr.TagChange(j)
			_, err = s.stmt[putTagChange].Exec(storeID, tc.Key, tc.Value, tc.Time.Format(formatISO8601), r.Group(), rr.Rule())
			if err != nil {
				return err
			}
		}
	}

	// Commit the transaction and return.
	err = tx.Commit()
	if err != nil {
		return err
	}
	tx = nil
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

CREATE TABLE IF NOT EXISTS tag(
    id 				INTEGER	PRIMARY KEY,
	message_id		TEXT 	NOT NULL,
	"key"       	TEXT 	NOT NULL,
	"value"     	TEXT,
	create_time 	TEXT 	NOT NULL,
	create_group 	TEXT 	NOT NULL,
	create_rule     TEXT    NOT NULL, 
	update_time 	TEXT,
	update_group    TEXT,
	update_rule     TEXT,

	FOREIGN KEY(message_id) REFERENCES message(id) 
);

CREATE UNIQUE INDEX IF NOT EXISTS iu_tag_on_message_id_key
                 ON tag(message_id, "key");

CREATE TABLE IF NOT EXISTS group_eval(
    id           INTEGER PRIMARY KEY,
	message_id   TEXT    NOT NULL,
	"group"      TEXT    NOT NULL,
	start_time   TEXT    NOT NULL,
	end_time     TEXT    NOT NULL,
	seconds      REAL    NOT NULL,
	stop         INTEGER,
	err          TEXT,

	FOREIGN KEY(message_id) REFERENCES message(id)
);

CREATE INDEX IF NOT EXISTS i_group_eval_on_message_id_id
          ON group_eval(message_id, id);

CREATE TABLE IF NOT EXISTS rule_eval(
    id            	INTEGER PRIMARY KEY,
    group_eval_id	INTEGER NOT NULL,
	rule         	TEXT    NOT NULL,
	start_time   	TEXT    NOT NULL,
	end_time     	TEXT    NOT NULL,
	seconds      	REAL    NOT NULL,
	stop         	INTEGER,
	err          	TEXT,

	FOREIGN KEY(group_eval_id) REFERENCES group_eval(id)
);

CREATE INDEX IF NOT EXISTS i_rule_eval_on_group_eval_id_id
          ON rule_eval(group_eval_id, id);
`)
	return err
}

const (
	getMetadataSampled stmt = iota
	getMetadataTags
	putMessage
	putGroupEvalRecord
	putRuleEvalRecord
	putTagChange
	numStmt

	formatISO8601 = "2006-01-02T15:04:05.999Z07:00"
)

var (
	stmtText = [numStmt]string{
		`SELECT is_sampled FROM message WHERE id = :id`,
		`SELECT "key", "value" FROM tag WHERE message_id = :id AND "value" IS NOT NULL`,
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
		`INSERT INTO group_eval(message_id, "group", start_time, end_time, seconds, stop, err)
			  VALUES (:message_id, :group, :start_time, :end_time, :seconds, :stop, :err)`,
		`INSERT INTO rule_eval(group_eval_id, rule, start_time, end_time, seconds, stop, err)
    		  VALUES (:group_eval_id, :rule, :start_time, :end_time, :seconds, :stop, :err)`,
		`INSERT INTO tag(message_id, "key", "value", create_time, create_group, create_rule)
    		  VALUES (:message_id, :key, :value, :time, :group, :rule)
    		      ON CONFLICT(message_id, "key") DO
    	  UPDATE SET "value" = :value, update_time = :time, update_group = :group, update_rule = :rule`,
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
