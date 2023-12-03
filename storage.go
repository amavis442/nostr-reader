package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nbd-wtf/go-nostr"
)

/**
 * We neede an active database connection object.
 * The filter is used for certain words in de posts we want to filter out, because they can be spam
 */
type Storage struct {
	Db     *sql.DB
	Filter []string
	Count  int64
}

type DbConfig struct {
	User     string
	Password string
	Dbname   string
	Port     int
	Host     string
}

/**
 * Since the above structs should be in sync with the database tables they represent.
 * I put the create statement of the database here even when it more a database thing which is storage.go.
 * Maybe change it later.
 */
const CreateQuery string = `
 CREATE TABLE IF NOT EXISTS events (
	 id SERIAL Primary Key,
	 event_id TEXT UNIQUE, 
	 pubkey TEXT, 
	 kind INTEGER, 
	 event_created_at INTEGER, 
	 content TEXT, 
	 tags_full TEXT, 
	 ptags text[],
	 etags text[],
	 sig TEXT,
	 raw TEXT,
	 garbage boolean DEFAULT false,
	 created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
 );
 CREATE INDEX IF NOT EXISTS idx_events_pubkey ON events (pubkey);
 
 CREATE TABLE IF NOT EXISTS profiles (
	 id SERIAL Primary Key,
	 pubkey VARCHAR UNIQUE, 
	 name TEXT,
	 about TEXT,
	 picture TEXT,
	 website TEXT,
	 nip05 TEXT,
	 lud16 TEXT,
	 display_name TEXT,
	 raw TEXT,
	 profile_created_at INTEGER,
	 created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
 );
 CREATE INDEX IF NOT EXISTS idx_profile_pubkey ON profiles (pubkey);
 
 CREATE TABLE IF NOT EXISTS block_pubkeys (
	 id SERIAL Primary Key, 
	 pubkey VARCHAR UNIQUE, 
	 created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
 );
 CREATE INDEX IF NOT EXISTS idx_block_pubkeys_pubkey ON block_pubkeys (pubkey);
 
 CREATE TABLE IF NOT EXISTS follow_pubkeys (
	 id SERIAL Primary Key,
	 pubkey VARCHAR UNIQUE, 
	 created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
 );
 CREATE INDEX IF NOT EXISTS idx_follow_pubkeys_pubkey ON follow_pubkeys (pubkey);
 
 CREATE TABLE IF NOT EXISTS seen (
	 id SERIAL Primary Key,
	 event_id VARCHAR UNIQUE, 
	 created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
 );
 CREATE INDEX IF NOT EXISTS seen_by_event_id ON seen (event_id);
 
 CREATE TABLE IF NOT EXISTS tree (
	 id SERIAL Primary Key,
	 event_id VARCHAR,
	 root_event_id VARCHAR,
	 reply_event_id VARCHAR, 
	 created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
 );
 `

func (st *Storage) CheckError(err error) {
	if err != nil {
		panic(err)
	}
}
func (st *Storage) CreateTables() {
	_, err := st.Db.Exec(CreateQuery)
	if err != nil {
		panic(err)
	}
}

/**
 * Connect to postgresql database
 */
func (st *Storage) Connect(cfg *Config) {
	// connection string
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.Dbname)

	var err error
	// open database
	st.Db, err = sql.Open("postgres", dsn)
	st.CheckError(err)

	fmt.Println("Connected!")
}

func (st *Storage) Close() {
	st.Db.Close()
}

/**
 * Save user profiles for easy lookup
 */
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
			log.Println(err.Error(), ev.Content)
			//panic(err)
			continue
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

/**
 * Save the events, mostly notes. Ignore duplicate events based on unique event id
 * This will normalize the content tag of the events with all the unwanted markup (Myaby put this in a helper function)
 */
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
		if len(ev.PubKey) == 64 {
			pubkeys = append(pubkeys, ev.PubKey)
		} else {
			log.Println("Incorrect pubkey to long max 64: ", ev.PubKey)
		}
		ptags = ptags[:0]
		etags = etags[:0]
		ptagsNum := 0
		etagsNum := 0

		tree.RootTag = ""
		tree.ReplyTag = ""
		for _, tag := range ev.Tags {
			switch {
			case tag[0] == "e":
				if len(tag) < 1 || len(tag[1]) != 64 {
					continue
				} else {
					etags = append(etags, tag[1])
					etagsNum = etagsNum + 1
				}
				if len(tag) == 4 && tag[3] == "root" {
					tree.RootTag = tag[1]
				}
				if len(tag) == 4 && tag[3] == "reply" {
					tree.ReplyTag = tag[1]
				}
			case tag[0] == "p":
				if len(tag) < 1 || len(tag[1]) != 64 {
					log.Println("P# tag not valid: ", tag)
					continue
				} else {
					log.Println("Adding pubkey from p# tag: ", tag[1])
					ptags = append(ptags, tag[1])
					pubkeys = append(pubkeys, tag[1])
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
			if matched {
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
		etags = etags[0:etagsSliceSize] // Same story as with ptags.
		if _, err := stmt.Exec(ev.ID, ev.PubKey, ev.Kind, ev.CreatedAt, ev.Content, string(tagJson), ev.Sig, ev.String(), pq.Array(ptags), pq.Array(etags), Garbage); err != nil {
			log.Println(ev.String(), err)
			panic(err)
		}

		if len(tree.RootTag) > 0 {
			log.Println("Roottag is: ", tree.RootTag)

			tx.Exec(treeQry, ev.ID, tree.RootTag, tree.ReplyTag)
		}
	}

	log.Println("Ready to save events")
	if err := tx.Commit(); err != nil {
		panic(err)
	}

	return pubkeys
}

/**
 * Get a limitted amount of stored events
 */
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

/**
 * Do not show all data in an endless scrol page, but paginate it for easy access
 * and ignore the garbage tagged posts
 */
func (st *Storage) GetEventPagination(p *Pagination) error {
	tx, err := st.Db.Begin()
	if err != nil {
		panic(err)
	}

	var (
		recordCount int64
		recordId    int64
	)
	//Only root events.
	mainQry := `SELECT e.id, e.event_id, e.pubkey, e.kind, e.event_created_at, e.content, e.tags_full, e.etags, e.ptags, e.sig, u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name FROM events e 
	LEFT JOIN profiles u ON (u.pubkey = e.pubkey ) 
	LEFT JOIN block_pubkeys b on (b.pubkey = e.pubkey) 
	LEFT JOIN seen s on (s.event_id = e.event_id)
	WHERE e.kind = 1 AND e.etags='{}' AND b.pubkey IS NULL AND s.event_id IS NULL AND e.garbage = false`
	if p.Since != 0 {
		since := time.Now().Unix() - int64(p.Since*60*60*24)
		mainQry = mainQry + ` AND e.event_created_at > ` + fmt.Sprintf("%d", since)
	}

	countQry := `SELECT COUNT(*) AS cnt FROM (SELECT DISTINCT id, event_id, event_created_at FROM (` + mainQry + `) resultTable) tbl`
	log.Println(countQry)

	selectIdQry := `SELECT id FROM (SELECT DISTINCT id, event_id, event_created_at FROM ( ` + mainQry + `) resultInnerTable ORDER BY event_created_at DESC) tbl  LIMIT ` + strconv.Itoa(p.Limit)
	if p.Offset > 0 {
		selectIdQry = selectIdQry + ` OFFSET ` + fmt.Sprintf("%d", p.Offset)
	}
	selectIdQry = selectIdQry + `;`
	log.Println(selectIdQry)

	selectQry := mainQry + ` AND e.id IN (`

	tx.QueryRow(countQry).Scan(&recordCount)
	p.SetTotal(recordCount)
	p.SetTo()
	if recordCount < 1 {
		return nil
	}

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
		return nil
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
		return nil
	}

	p.Data = events
	return nil
	//page := &Pagination{Data: events, Total: recordCount, Pages: pages}
}

/**
 * Find an event based on an unique event id
 */
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

/**
 * Some users just posting garbage, so we try to block those by putting them on the naugthy list
 */
func (st *Storage) BlockPubkey(pubkey string) error {
	_, err := st.Db.Exec(`INSERT INTO "block_pubkeys" (pubkey, created_at) VALUES ($1, NOW()) ON CONFLICT (pubkey) DO NOTHING;`, pubkey)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

/**
 * And there are user we like, so put them on the good list
 */
func (st *Storage) FollowPubkey(pubkey string) error {
	_, err := st.Db.Exec(`INSERT INTO "follow_pubkeys" (pubkey, created_at) VALUES ($1, NOW()) ON CONFLICT (pubkey) DO NOTHING;`, pubkey)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (st *Storage) getLastTimeStamp() int64 {
	var createdAt int64
	row := st.Db.QueryRow("SELECT MAX(created_at) as MaxCreated FROM events")
	row.Scan(&createdAt)

	return createdAt
}
