package main

import (
	"database/sql"
	"encoding/json"
	"log"

	"github.com/nbd-wtf/go-nostr"
)

func StoreEvent(db *sql.DB, ev *nostr.Event) (int64, error) {
	var qry = `INSERT OR IGNORE INTO events (id, pubkey, kind, created_at, content,tags_full, sig) VALUES (?, ?, ?, ?, ?, ?, ?)`

	tagJson, err := json.Marshal(ev.Tags)
	if err != nil {
		log.Fatal(err)
	}
	result, ok := db.Exec(qry, ev.ID, ev.PubKey, ev.Kind, ev.CreatedAt, ev.Content, string(tagJson), ev.Sig)

	if ok != nil {
		return 0, ok
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}
