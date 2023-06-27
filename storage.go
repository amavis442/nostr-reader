package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nbd-wtf/go-nostr"
)

type Storage struct {
	Db     *sql.DB
	Filter []string
	Count  int64
}

func (st *Storage) CheckError(err error) {
	if err != nil {
		panic(err)
	}
}

func (st *Storage) Connect(cfg *Config) {
	// connection string
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.Dbname)

	var err error
	// open database
	st.Db, err = sql.Open("postgres", dsn)
	st.CheckError(err)

	fmt.Println("Connected!")
}

func (st *Storage) SaveProfiles(evs []*nostr.Event) {
	var qry = `INSERT INTO "profiles" ("pubkey", "name","about", "picture",  "website", "nip05",
	"lud16", "display_name", "raw", "profile_created_at", "created_at")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW()) ON CONFLICT (pubkey) DO NOTHING;`

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var tx *sql.Tx
	tx, err := st.Db.Begin()
	if err != nil {
		panic(err)
	}
	for _, ev := range evs {
		var data Profile
		err = json.Unmarshal([]byte(ev.Content), &data)
		if err != nil {
			panic(err)
		}

		_, err = tx.Exec(qry, ev.PubKey, data.Name, data.About, data.Picture, data.Website, data.Nip05, data.Lud16, data.DisplayName, ev.String(), ev.CreatedAt)
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.Printf("update profile: unable to rollback: %v", rollbackErr)
			}
			log.Println(err)
			ctx.Done()
		}
		log.Println("Profile: ", data.Name)
	}
	if err := tx.Commit(); err != nil {
		panic(err)
	}
}

func (st *Storage) SaveEvents(evs []*nostr.Event) []string {
	var qry = `INSERT INTO "events" ("event_id", "pubkey", "kind", "event_created_at", "content", "tags_full" , "sig" , "raw" , "ptags" , "etags", "garbage", "created_at") 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW()) ON CONFLICT (event_id) DO NOTHING;`

	var treeQry = `INSERT INTO tree ("event_id","root_event_id", "reply_event_id", "created_at") VALUES ($1, $2, $3, NOW())`

	var pubkeys = make([]string, 0)

	tx, err := st.Db.Begin()
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
		for _, f := range st.Filter {
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

			tx.Exec(treeQry, ev.ID, tree.RootTag, tree.ReplyTag)
		}
	}

	log.Println("Ready to save events")
	if err := tx.Commit(); err != nil {
		panic(err)
	}
	return pubkeys
}

func (st *Storage) GetEvents(limit int) (*[]Event, error) {
	tx, err := st.Db.Begin()
	if err != nil {
		panic(err)
	}
	rows, err := tx.Query(`SELECT e.event_id, e.pubkey, e.kind, e.event_created_at, e.content, e.tags_full, e.etags, e.ptags, e.sig, u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name FROM events e LEFT JOIN profiles u ON (u.pubkey = e.pubkey ) LEFT JOIN block_pubkeys b on (b.pubkey = e.pubkey) LEFT JOIN seen s on (s.event_id = e.event_id)
	WHERE e.kind = 1 AND b.pubkey IS NULL AND s.event_id IS NULL AND e.garbage = false ORDER BY e.event_created_at DESC LIMIT $1`, limit)

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

		if err := rows.Scan(&event.EventID, &event.Pubkey, &event.Kind, &event.EventCreatedAt, &event.Content, &event.TagsFull, pq.Array(&event.Etags), pq.Array(&event.Ptags), &event.Sig,
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

type Pagination struct {
	Data        []Event `json:"data"`
	Pages       int64   `json:"pages"`
	Total       int64   `json:"total"`
	Limit       int     `json:"limit"`
	PerPage     int     `json:"per_page"`
	Offset      int     `json:"offset"`
	CurrentPage int     `json:"current_page"`
	LastPage    int64   `json:"last_page"`
	From        int     `json:"from"`
	To          int     `json:"to"`
}

func (st *Storage) GetEventPagination(offset int, limit int) (*Pagination, error) {
	tx, err := st.Db.Begin()
	if err != nil {
		panic(err)
	}

	var (
		recordCount int64
		recordId    int64
	)

	mainQry := `SELECT e.id, e.event_id, e.pubkey, e.kind, e.event_created_at, e.content, e.tags_full, e.etags, e.ptags, e.sig, u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name FROM events e LEFT JOIN profiles u ON (u.pubkey = e.pubkey ) LEFT JOIN block_pubkeys b on (b.pubkey = e.pubkey) LEFT JOIN seen s on (s.event_id = e.event_id)
	WHERE e.kind = 1 AND b.pubkey IS NULL AND s.event_id IS NULL AND e.garbage = false`

	countQry := `SELECT COUNT(*) AS cnt FROM (SELECT DISTINCT id, event_id, event_created_at FROM (` + mainQry + `) resultTable) tbl`
	log.Println(countQry)

	selectIdQry := `SELECT id FROM (SELECT DISTINCT id, event_id, event_created_at FROM ( ` + mainQry + `) resultInnerTable ORDER BY event_created_at DESC) tbl  LIMIT ` + strconv.Itoa(limit)
	if offset > 0 {
		selectIdQry = selectIdQry + ` OFFSET ` + fmt.Sprintf("%d", offset)
	}
	selectIdQry = selectIdQry + `;`
	log.Println(selectIdQry)

	selectQry := mainQry + ` AND e.id IN (`

	tx.QueryRow(countQry).Scan(&recordCount)
	pages := int64(math.Floor(float64(recordCount) / float64(limit)))

	rowsIds, err := tx.Query(selectIdQry)
	if err != nil {
		log.Fatal(err)
	}
	for rowsIds.Next() {
		rowsIds.Scan(&recordId)
		selectQry = selectQry + fmt.Sprintf("%d", recordId) + ","
	}
	rowsIds.Close()

	finalQry := selectQry[:len(selectQry)-1]
	finalQry = finalQry + ") ORDER BY event_created_at DESC;"
	log.Println(finalQry)
	rows, err := tx.Query(finalQry)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var event Event
		var id int
		var name sql.NullString
		var about sql.NullString
		var picture sql.NullString

		var website sql.NullString
		var nip05 sql.NullString
		var lud16 sql.NullString
		var displayname sql.NullString

		if err := rows.Scan(&id, &event.EventID, &event.Pubkey, &event.Kind, &event.EventCreatedAt, &event.Content, &event.TagsFull, pq.Array(&event.Etags), pq.Array(&event.Ptags), &event.Sig,
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

	page := &Pagination{Data: events, Total: recordCount, Pages: pages, Limit: limit, Offset: offset}
	return page, nil
}

func (st *Storage) FindEvent(id string) Event {
	var qry = `SELECT e.event_id, e.pubkey, e.kind, e.event_created_at, e.content, e.tags_full, e.etags, e.ptags, e.sig, u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name
	FROM events e LEFT JOIN profiles u ON (u.pubkey = e.pubkey ) LEFT JOIN block_pubkeys b on (b.pubkey = e.pubkey) 
	WHERE e.event_id = $1`
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

	err := st.Db.QueryRow(qry, id).Scan(&event.EventID, &event.Pubkey, &event.Kind, &event.EventCreatedAt, &event.Content, &event.TagsFull, pq.Array(&event.Etags), pq.Array(&event.Ptags), &event.Sig,
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

func (st *Storage) BlockPubkey(pubkey string) error {
	_, err := st.Db.Exec(`INSERT INTO "block_pubkeys" (pubkey, created_at) VALUES ($1, NOW()) ON CONFLICT (pubkey) DO NOTHING;`, pubkey)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (st *Storage) FollowPubkey(pubkey string) error {
	_, err := st.Db.Exec(`INSERT INTO "follow_pubkeys" (pubkey, created_at) VALUES ($1, NOW()) ON CONFLICT (pubkey) DO NOTHING;`, pubkey)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}
