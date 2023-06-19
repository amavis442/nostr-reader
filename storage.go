package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nbd-wtf/go-nostr"
)

type Storage struct {
	Db     *sql.DB
	Filter []string
}

func (db *Storage) CheckError(err error) {
	if err != nil {
		panic(err)
	}
}

func (db *Storage) Connect(cfg *Config) {
	// connection string
	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.Dbname)

	var err error
	// open database
	db.Db, err = sql.Open("postgres", psqlconn)
	db.CheckError(err)

	fmt.Println("Connected!")
}

func (db *Storage) SaveProfiles(evs []*nostr.Event) {
	var qry = `INSERT INTO "users" ("pubkey", "name","about", "picture",  "website", "nip05",
	"lud16", "display_name", "raw", "created_at")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT (pubkey) DO NOTHING;`

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var tx *sql.Tx
	tx, err := db.Db.Begin()
	if err != nil {
		panic(err)
	}
	for _, ev := range evs {
		var data Profile
		err = json.Unmarshal([]byte(ev.Content), &data)
		if err != nil {
			panic(err)
		}

		_, err = tx.Exec(qry, ev.PubKey, data.Name, data.About, data.Picture, data.Website, data.Nip05, data.Lud16, data.DisplayName, ev.String(), time.Now().Unix())
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.Fatalf("update users: unable to rollback: %v", rollbackErr)
			}
			log.Fatal(err)
			ctx.Done()
		}
		log.Println("User: ", data.Name)
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
}

func (db *Storage) SaveEvents(evs []*nostr.Event) []string {
	var qry = `INSERT INTO "events" ("id", "pubkey", "kind", "created_at", "content" , "tags_full" , "sig" , "raw" , "ptags" , "etags", "garbage") 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT (id) DO NOTHING;`

	var treeQry = `INSERT INTO tree ("event_id","root_event_id", "reply_event_id", "created_at") VALUES ($1, $2, $3, $4)`

	var pubkeys = make([]string, 0)

	tx, err := db.Db.Begin()
	if err != nil {
		panic(err)
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	stmt, err := tx.Prepare(qry)
	if err != nil {
		panic(err)
	}
	defer stmt.Close() // Prepared statements take up server resources and should be closed after use.

	ptags, etags := make([]string, 0), make([]string, 0)

	type Tree struct {
		RootTag  string
		ReplyTag string
	}
	var tree Tree
	for _, ev := range evs {
		log.Println("Event ID: ", ev.ID)
		pubkeys = append(pubkeys, fmt.Sprintf("%x", ev.PubKey))

		ptags = ptags[:0]
		etags = etags[:0]
		ptagsNum := 0
		etagsNum := 0

		tree.RootTag = ""
		tree.ReplyTag = ""
		for _, tag := range ev.Tags {
			switch {
			case tag[0] == "e":
				if b, e := hex.DecodeString(tag[1]); e != nil || len(b) != 32 {
					continue
				} else {
					etags = append(etags, fmt.Sprintf("%x", b))
					etagsNum = etagsNum + 1
				}
				if len(tag) == 4 && tag[3] == "root" {
					tree.RootTag = tag[1]
				}
				if len(tag) == 4 && tag[3] == "reply" {
					tree.ReplyTag = tag[1]
				}
			case tag[0] == "p":
				if b, e := hex.DecodeString(tag[1]); e != nil || len(b) != 32 {
					continue
				} else {
					ptags = append(ptags, fmt.Sprintf("%x", b))
					pubkeys = append(pubkeys, fmt.Sprintf("%x", b))
					ptagsNum = ptagsNum + 1
				}
			}
		}

		tagJson, err := json.Marshal(ev.Tags)
		if err != nil {
			log.Println(err)
		}

		p := bluemonday.StrictPolicy()

		// The policy can then be used to sanitize lots of input and it is safe to use the policy in multiple goroutines
		ev.Content = p.Sanitize(ev.Content)
		ev.Content = strings.ReplaceAll(ev.Content, "&#39;", "'")
		ev.Content = strings.ReplaceAll(ev.Content, "&#34;", "\"")
		ev.Content = strings.ReplaceAll(ev.Content, "&lt;", "<")
		ev.Content = strings.ReplaceAll(ev.Content, "&gt;", ">")
		ev.Content = strings.ReplaceAll(ev.Content, "&amp;", "&")
		ev.Content = strings.ReplaceAll(ev.Content, "<br>", "\n")
		ev.Content = strings.ReplaceAll(ev.Content, "<br/>", "\n")

		var Garbage bool = false
		for _, f := range db.Filter {
			matched, _ := regexp.MatchString(f, ev.Content)
			if matched == true {
				Garbage = true
				log.Println("Got a match", ev.Content)
			}
		}

		log.Println("Add to transaction")
		ev.Content = strings.ReplaceAll(ev.Content, "\u0000", "")
		ptagsSliceSize := ptagsNum
		if ptagsNum > 8 {
			ptagsSliceSize = 8
		}
		etagsSliceSize := etagsNum
		if etagsNum > 8 {
			etagsSliceSize = 8
		}
		ptags = ptags[0:ptagsSliceSize] // Idiots who put a lot of ptags in it. Bad clients
		etags = etags[0:etagsSliceSize] // Same store as with ptags.
		if _, err := stmt.Exec(ev.ID, ev.PubKey, ev.Kind, ev.CreatedAt, ev.Content, string(tagJson), ev.Sig, ev.String(), pq.Array(ptags), pq.Array(etags), Garbage); err != nil {
			log.Println(ev.String(), err)
			panic(err)
		}

		if len(tree.RootTag) > 0 {
			log.Println("Roottag is ", tree.RootTag)

			tx.Exec(treeQry, ev.ID, tree.RootTag, tree.ReplyTag, time.Now().Unix())
		}
	}

	log.Println("Ready to save events")
	if err := tx.Commit(); err != nil {
		panic(err)
	}
	return pubkeys
}

func (db *Storage) GetEvents(limit int) (*[]Event, error) {
	tx, err := db.Db.Begin()
	if err != nil {
		panic(err)
	}

	rows, err := tx.Query(`SELECT e.id, e.pubkey, e.kind, e.created_at, e.content, e.tags_full, e.etags, e.ptags, e.sig, u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name
	FROM events e LEFT JOIN users u ON (u.pubkey = e.pubkey ) LEFT JOIN blockusers b on (b.pubkey = e.pubkey) LEFT JOIN seen s on (s.event_id = e.id)
	WHERE e.kind = 1 AND b.pubkey IS NULL AND s.event_id IS NULL AND e.garbage = false ORDER BY e.created_at DESC LIMIT $1`, limit)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var event Event
		var name sql.NullString
		var about sql.NullString
		var picture sql.NullString

		var website sql.NullString
		var nip05 sql.NullString
		var lud16 sql.NullString
		var displayname sql.NullString

		if err := rows.Scan(&event.ID, &event.Pubkey, &event.Kind, &event.CreatedAt, &event.Content, &event.Tags_full, pq.Array(&event.Etags), pq.Array(&event.Ptags), &event.Sig,
			&name, &about, &picture, &website, &nip05, &lud16, &displayname); err != nil {
			panic(err)
		}
		if name.Valid {
			event.Profile.Name = name.String
		} else {
			event.Profile.Name = event.Pubkey
		}
		if about.Valid {
			event.Profile.About = about.String
		}
		if picture.Valid {
			event.Profile.Picture = picture.String
		}

		if website.Valid {
			event.Profile.Website = website.String
		}
		if nip05.Valid {
			event.Profile.Nip05 = nip05.String
		}
		if lud16.Valid {
			event.Profile.Lud16 = lud16.String
		}
		if displayname.Valid {
			event.Profile.DisplayName = displayname.String
		}

		/* WIP
		var tags nostr.Tags = json.Unmarshal(event.Tags_full.(nostr.Tag))
		if tags.GetFirst("e") != nil {
			continue
		}
		*/
		events = append(events, event)
	}
	// Check for errors from iterating over rows.
	if err := rows.Err(); err != nil {
		log.Println(err)
		return nil, err
	}

	return &events, nil
}

func (db *Storage) FindEvent(id string) Event {
	var qry = `SELECT e.id, e.pubkey, e.kind, e.created_at, e.content, e.tags_full, e.etags, e.ptags, e.sig, u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name
	FROM events e LEFT JOIN users u ON (u.pubkey = e.pubkey ) LEFT JOIN blockusers b on (b.pubkey = e.pubkey) 
	WHERE e.id = $1`
	//qry = qry + "'" + id + "'"
	log.Println(qry)

	var event Event
	var name sql.NullString
	var about sql.NullString
	var picture sql.NullString

	var website sql.NullString
	var nip05 sql.NullString
	var lud16 sql.NullString
	var displayname sql.NullString

	err := db.Db.QueryRow(qry, id).Scan(&event.ID, &event.Pubkey, &event.Kind, &event.CreatedAt, &event.Content, &event.Tags_full, pq.Array(&event.Etags), pq.Array(&event.Ptags), &event.Sig,
		&name, &about, &picture, &website, &nip05, &lud16, &displayname)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("404 no event with id %s\n", id)
	case err != nil:
		log.Fatalf("502 query error: %v\n", err)
	default:
		log.Println("200 Event: ", event)
	}

	return event
}

func (db *Storage) BlockUser(pubkey string) error {
	_, err := db.Db.Exec(`INSERT INTO "blockusers" (pubkey, created_at) VALUES ($1, $2) ON CONFLICT (pubkey) DO NOTHING;`, pubkey, time.Now().Unix())
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (db *Storage) FollowUser(pubkey string) error {
	_, err := db.Db.Exec(`INSERT INTO "followusers" (pubkey, created_at) VALUES ($1, $2) ON CONFLICT (pubkey) DO NOTHING;`, pubkey, time.Now().Unix())
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}
