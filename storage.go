package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nbd-wtf/go-nostr"
)

/**
 * We neede an active database connection object.
 * The filter is used for certain words in de posts we want to filter out, because they can be spam
 */
type Storage struct {
	DbPool *pgxpool.Pool
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
	 event_id TEXT UNIQUE NOT NULL, 
	 pubkey TEXT NOT NULL, 
	 kind INTEGER NOT NULL, 
	 event_created_at INTEGER NOT NULL, 
	 content TEXT, 
	 tags_full TEXT, 
	 ptags text[],
	 etags text[],
	 sig TEXT,
	 garbage boolean DEFAULT false,
	 raw json,
	 created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
 );
 CREATE INDEX IF NOT EXISTS idx_events_pubkey ON events (pubkey);
 CREATE INDEX IF NOT EXISTS idx_events_ptags ON events USING gin (etags);
 CREATE INDEX IF NOT EXISTS idx_events_etags ON events USING gin (ptags);
 
 CREATE TABLE IF NOT EXISTS profiles (
	 id SERIAL Primary Key,
	 pubkey VARCHAR UNIQUE NOT NULL, 
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
	 pubkey VARCHAR UNIQUE NOT NULL, 
	 created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
 );
 CREATE INDEX IF NOT EXISTS idx_block_pubkeys_pubkey ON block_pubkeys (pubkey);
 
 CREATE TABLE IF NOT EXISTS follow_pubkeys (
	 id SERIAL Primary Key,
	 pubkey VARCHAR UNIQUE NOT NULL, 
	 created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
 );
 CREATE INDEX IF NOT EXISTS idx_follow_pubkeys_pubkey ON follow_pubkeys (pubkey);
 
 CREATE TABLE IF NOT EXISTS seen (
	 id SERIAL Primary Key,
	 event_id VARCHAR UNIQUE NOT NULL, 
	 created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
 );
 CREATE INDEX IF NOT EXISTS seen_by_event_id ON seen (event_id);
 
 CREATE TABLE IF NOT EXISTS tree (
	 id SERIAL Primary Key,
	 event_id VARCHAR UNIQUE NOT NULL,
	 root_event_id VARCHAR,
	 reply_event_id VARCHAR, 
	 created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
 );

 `

func (st *Storage) CheckError(err error) {
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}
}
func (st *Storage) CreateTables(ctx context.Context) error {
	_, err := st.DbPool.Exec(ctx, CreateQuery)
	return err
}

/**
 * Connect to postgresql database
 */
func (st *Storage) Connect(ctx context.Context, cfg *Config) error {
	// connection string
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.Dbname)

	var err error
	st.DbPool, err = pgxpool.New(ctx, connStr)
	fmt.Println("Connected!")
	return err
}

func (st *Storage) Close() {
	st.DbPool.Close()
}

/**
 * Save user profiles for easy lookup
 */
func (st *Storage) SaveProfiles(ctx context.Context, evs []*nostr.Event) {
	var qry = `INSERT INTO "profiles" ("pubkey", "name","about", "picture",  "website", "nip05",
	"lud16", "display_name", "raw", "profile_created_at", "created_at")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW()) ON CONFLICT (pubkey) DO 
		UPDATE SET "pubkey" = $1, "name" = $2,"about" = $3, "picture" = $4,  "website" = $5, "nip05" = $6,
		"lud16" = $7, "display_name" = $8, "raw" = $9, "profile_created_at" = $10, "created_at" = NOW()
		
		;`

	var tx pgx.Tx
	tx, err := st.DbPool.Begin(ctx)
	if err != nil {
		log.Println(err.Error())
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

		_, err = tx.Exec(ctx, qry, ev.PubKey, data.Name, data.About, data.Picture, data.Website, data.Nip05, data.Lud16, data.DisplayName, ev.String(), ev.CreatedAt)
		if err != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				log.Printf("update profile: unable to rollback: %v", rollbackErr)
			}
			log.Println(err)
			ctx.Done()
		}
		fmt.Println("Profile: ", data.Name)

		/*
			if data.Nip05 == "" {
				st.BlockPubkey(ctx, ev.PubKey)
			}
		*/
		// Should be in a dynamic list, so you can add to it or remove items.
		if data.Picture != "" && len(data.Picture) > len("https://randomuser.me") && data.Picture[0:len("https://randomuser.me")] == "https://randomuser.me" {
			st.BlockPubkey(ctx, ev.PubKey)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		log.Println(err.Error())
		panic(err)
	}
}

/**
 * Save the events, mostly notes. Ignore duplicate events based on unique event id
 * This will normalize the content tag of the events with all the unwanted markup (Myaby put this in a helper function)
 */
func (st *Storage) SaveEvents(ctx context.Context, evs []*nostr.Event) []string {
	var qry = `INSERT INTO "events" ("event_id", "pubkey", "kind", "event_created_at", "content", "tags_full" , "sig" , "ptags" , "etags", "garbage", "raw", "created_at") 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW()) ON CONFLICT (event_id) DO NOTHING;`

	var treeQry = `INSERT INTO tree ("event_id","root_event_id", "reply_event_id", "created_at") VALUES ($1, $2, $3, NOW()) ON CONFLICT (event_id) DO NOTHING;`

	tx, err := st.DbPool.Begin(ctx)
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		} else {
			fmt.Println("Ready to save events")

			tx.Commit(ctx)
		}
	}()

	var pubkeys = make([]string, 0)
	ptags, etags := make([]string, 0), make([]string, 0)

	type Tree struct {
		RootTag  string
		ReplyTag string
	}
	var tree Tree
	for _, ev := range evs {
		fmt.Println("Event ID: ", ev.ID)
		if len(ev.PubKey) == 64 {
			pubkeys = append(pubkeys, ev.PubKey)
		} else {
			fmt.Println("Incorrect pubkey to long max 64: ", ev.PubKey)
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
					fmt.Println("P# tag not valid: ", tag)
					continue
				} else {
					fmt.Println("Adding pubkey from p# tag: ", tag[1])
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
				fmt.Println("Got a match", ev.Content)
			}
		}

		fmt.Println("Add to transaction")
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

		jsonbuf := bytes.NewBuffer(nil)
		jsonbuf.Reset()
		enc := json.NewEncoder(jsonbuf)
		// turn off stupid go json encoding automatically doing HTML escaping...
		enc.SetEscapeHTML(false)
		if err := enc.Encode(ev); err != nil {
			log.Println(err)
			return []string{}
		}

		if _, err := tx.Exec(ctx, qry, ev.ID, ev.PubKey, ev.Kind, ev.CreatedAt, ev.Content, string(tagJson), ev.Sig, ptags, etags, Garbage, jsonbuf.Bytes()); err != nil {
			log.Println(qry)
			log.Println(err.Error())
			log.Println(ev.String())
			log.Println(jsonbuf.Bytes())
			panic(err)
		}

		if len(tree.RootTag) > 0 {
			fmt.Println("Roottag is: ", tree.RootTag)

			tx.Exec(ctx, treeQry, ev.ID, tree.RootTag, tree.ReplyTag)
		}
	}

	return pubkeys
}

/**
 * Get a limitted amount of stored events
 */
func (st *Storage) GetEvents(ctx context.Context, limit int) (*[]Event, error) {
	tx, err := st.DbPool.Begin(ctx)
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		} else {
			tx.Commit(ctx)
		}
	}()

	rows, err := tx.Query(ctx, `SELECT e.event_id, e.pubkey, e.kind, e.event_created_at, e.content, e.tags_full::json,e.sig, e.etags, e.ptags, u.name, u.about , u.picture,
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

		event.Event = &nostr.Event{}
		if err := rows.Scan(&event.Event.ID, &event.Event.PubKey, &event.Event.Kind, &event.Event.CreatedAt, &event.Event.Content, &event.Event.Tags, &event.Event.Sig,
			&event.Etags, &event.Ptags,
			&name, &about, &picture, &website, &nip05, &lud16, &displayname); err != nil {
			log.Println((err.Error()))
			panic(err)
		}

		if name.Valid {
			event.Profile.Name = name.String
		} else {
			event.Profile.Name = event.Event.PubKey
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
		event.Tree = 1
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
 *
 */
func (st *Storage) GetEventPagination(ctx context.Context, p *Pagination, follow bool) error {
	tx, err := st.DbPool.Begin(ctx)
	defer func() {
		if err != nil {
			tx.Rollback(ctx)
		} else {
			tx.Commit(ctx)
		}
	}()

	var (
		recordCount int64
		recordId    int64
	)

	//Only root events.
	mainQry := `SELECT e.id, e.event_id, e.pubkey, e.kind, e.event_created_at, e.content, e.tags_full::json, e.sig, e.etags, e.ptags,  
	u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name, f.pubkey follow
	FROM events e 
	LEFT JOIN profiles u ON (u.pubkey = e.pubkey ) 
	LEFT JOIN block_pubkeys b on (b.pubkey = e.pubkey) 
	LEFT JOIN seen s on (s.event_id = e.event_id)`

	if follow {
		mainQry += `
		JOIN follow_pubkeys f ON (f.pubkey = e.pubkey)
		`
	} else {
		mainQry += `
		LEFT JOIN follow_pubkeys f ON (f.pubkey = e.pubkey)
		`
	}
	mainQry += `WHERE e.kind = 1 AND e.etags='{}' AND b.pubkey IS NULL AND s.event_id IS NULL AND e.garbage = false`
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

	tx.QueryRow(ctx, countQry).Scan(&recordCount)
	p.SetTotal(recordCount)
	p.SetTo()
	if recordCount < 1 {
		return nil
	}

	rowsIds, err := tx.Query(ctx, selectIdQry)
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
	rows, err := tx.Query(ctx, finalQry)
	if err != nil {
		log.Println(err)
		return nil
	}
	defer rows.Close()

	/**
	 * Get all root notes
	 */
	events := make([]Event, 0)
	eventMap := make(map[string]Event)
	var keys []string
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
		var followed sql.NullString

		event.Event = &nostr.Event{}
		if err := rows.Scan(&id, &event.Event.ID, &event.Event.PubKey, &event.Event.Kind, &event.Event.CreatedAt, &event.Event.Content, &event.Event.Tags, &event.Event.Sig,
			&event.Etags, &event.Ptags,
			&name, &about, &picture, &website, &nip05, &lud16, &displayname, &followed); err != nil {
			log.Println(err.Error())
			panic(err)
		}

		event.RootId = event.Event.ID
		event.Tree = 1

		if name.Valid {
			event.Profile.Name = name.String
		} else {
			event.Profile.Name = event.Event.PubKey
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

		event.Profile.Followed = false
		if followed.Valid {
			event.Profile.Followed = true
		}

		//nostr.Event = json.Unmarshal()
		//sdk.ParseReferences(&nostr.Event{event})

		event.Children = make(map[string]Event, 0)
		eventMap[event.Event.ID] = event
		keys = append(keys, event.Event.ID) // Make sure the order stays the same @see https://go.dev/blog/maps
	}
	// Check for errors from iterating over rows.
	if err := rows.Err(); err != nil {
		log.Println(err)
		return nil
	}
	rows.Close()

	/**
	 * Get all child notes
	 */
	treeQry := `SELECT t.root_event_id, t.reply_event_id, 
	e.id, e.event_id, e.pubkey, e.kind, e.event_created_at, e.content,e.tags_full::json,e.sig, 
	e.etags, e.ptags , u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name, f.pubkey follow
	FROM tree t, events e 
	LEFT JOIN profiles u ON (u.pubkey = e.pubkey ) 
	LEFT JOIN block_pubkeys b on (b.pubkey = e.pubkey) 
	LEFT JOIN seen s on (s.event_id = e.event_id)
	LEFT JOIN follow_pubkeys f ON (f.pubkey = e.pubkey)
	WHERE root_event_id IN (`
	for k := range eventMap {
		treeQry = treeQry + `'` + k + `',`
	}
	treeQry = treeQry[:len(treeQry)-1] + `) AND e.event_id = t.event_id
	AND e.kind = 1 AND b.pubkey IS NULL AND s.event_id IS NULL AND e.garbage = false;`
	log.Println("Tree query: ", treeQry)
	var treeRows pgx.Rows
	treeRows, err = tx.Query(ctx, treeQry)
	if err != nil {
		log.Println(err)
	}
	for treeRows.Next() {
		var childEvent Event
		var id int
		var root_event_id string
		var reply_event_id sql.NullString
		var name sql.NullString
		var about sql.NullString
		var picture sql.NullString

		var website sql.NullString
		var nip05 sql.NullString
		var lud16 sql.NullString
		var displayname sql.NullString
		var followed sql.NullString
		childEvent.Event = &nostr.Event{}

		if err := treeRows.Scan(&root_event_id, &reply_event_id, &id,
			&childEvent.Event.ID, &childEvent.Event.PubKey, &childEvent.Event.Kind, &childEvent.Event.CreatedAt, &childEvent.Event.Content, &childEvent.Event.Tags, &childEvent.Event.Sig,
			&childEvent.Etags, &childEvent.Ptags, &name, &about, &picture,
			&website, &nip05, &lud16, &displayname, &followed); err != nil {
			log.Println(err.Error())
			panic(err)
		}

		childEvent.Tree = 2
		childEvent.RootId = root_event_id

		if name.Valid {
			childEvent.Profile.Name = name.String
		} else {
			childEvent.Profile.Name = childEvent.Event.PubKey
		}
		if about.Valid {
			childEvent.Profile.About = about.String
		}
		if picture.Valid {
			childEvent.Profile.Picture = picture.String
		}

		if website.Valid {
			childEvent.Profile.Website = website.String
		}
		if nip05.Valid {
			childEvent.Profile.Nip05 = nip05.String
		}
		if lud16.Valid {
			childEvent.Profile.Lud16 = lud16.String
		}
		if displayname.Valid {
			childEvent.Profile.DisplayName = displayname.String
		}

		childEvent.Profile.Followed = false
		if followed.Valid {
			childEvent.Profile.Followed = true
		}

		//event.Children = make(map[string]Event)
		if item, ok := eventMap[root_event_id]; ok {
			if reply_event_id.Valid && reply_event_id.String == "" {
				childEvent.Tree = 2
				item.Children[childEvent.Event.ID] = childEvent
				//item.Children = append(item.Children, event)
				eventMap[root_event_id] = item
			}
			/*if reply_event_id.Valid && reply_event_id.String <> ""{
				for _, child := range item.Children {
					if child.EventID == reply_event_id.String {
						if len(child.Children) < 1 {
							child.Children = make([]Event, 0)
						}
						child.Children = append(child.Children, event)
						eventMap[root_event_id] = item
					}
				}
			}
			*/
		}
	}
	if err := treeRows.Err(); err != nil {
		log.Println(err)
	}
	treeRows.Close()

	// Make sure the order stays the same
	for _, k := range keys {
		events = append(events, eventMap[k])
	}

	p.Data = events
	return nil
}

/**
 * Find an event based on an unique event id
 */
func (st *Storage) FindEvent(ctx context.Context, id string) (Event, error) {
	var qry = `SELECT e.event_id, e.pubkey, e.kind, e.event_created_at, e.content, e.tags_full::json, e.sig, e.etags, e.ptags, 
	u.name, u.about , u.picture, u.website, u.nip05, u.lud16, u.display_name
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
	event.Event = &nostr.Event{}

	err := st.DbPool.QueryRow(ctx, qry, id).Scan(&event.Event.ID, &event.Event.PubKey, &event.Event.Kind, &event.Event.CreatedAt, &event.Event.Content, &event.Event.Tags, &event.Event.Sig,
		&event.Etags, &event.Ptags, &name, &about, &picture, &website, &nip05, &lud16, &displayname)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("404 no event with id %s\n", id)
	case err != nil:
		log.Fatalf("502 query error: %v\n", err)
	default:
		log.Println("200 Event: ", event)
	}
	event.Tree = 1
	event.RootId = event.Event.ID
	event.Children = make(map[string]Event, 0)

	treeQry := `SELECT t.root_event_id, t.reply_event_id, 
	e.id, e.event_id, e.pubkey, e.kind, e.event_created_at, e.content,e.tags_full::json,e.sig, 
	e.etags, e.ptags , u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name, f.pubkey follow
	FROM tree t, events e 
	LEFT JOIN profiles u ON (u.pubkey = e.pubkey ) 
	LEFT JOIN block_pubkeys b on (b.pubkey = e.pubkey) 
	LEFT JOIN seen s on (s.event_id = e.event_id)
	LEFT JOIN follow_pubkeys f ON (f.pubkey = e.pubkey)
	WHERE root_event_id IN (` + `'` + event.Event.ID + `')` +
		` AND e.event_id = t.event_id
	AND e.kind = 1 AND b.pubkey IS NULL AND s.event_id IS NULL AND e.garbage = false;`

	var treeRows pgx.Rows
	treeRows, err = st.DbPool.Query(ctx, treeQry)
	if err != nil {
		log.Println(err)
	}
	for treeRows.Next() {
		var childEvent Event
		var id int
		var root_event_id string
		var reply_event_id sql.NullString
		var name sql.NullString
		var about sql.NullString
		var picture sql.NullString

		var website sql.NullString
		var nip05 sql.NullString
		var lud16 sql.NullString
		var displayname sql.NullString
		var followed sql.NullString
		childEvent.Event = &nostr.Event{}

		if err := treeRows.Scan(&root_event_id, &reply_event_id, &id,
			&childEvent.Event.ID, &childEvent.Event.PubKey, &childEvent.Event.Kind, &childEvent.Event.CreatedAt, &childEvent.Event.Content, &childEvent.Event.Tags, &childEvent.Event.Sig,
			&childEvent.Etags, &childEvent.Ptags, &name, &about, &picture,
			&website, &nip05, &lud16, &displayname, &followed); err != nil {
			log.Println(err.Error())
			panic(err)
		}

		childEvent.RootId = event.Event.ID
		childEvent.Tree = 2

		if name.Valid {
			childEvent.Profile.Name = name.String
		} else {
			childEvent.Profile.Name = event.Event.PubKey
		}
		if about.Valid {
			childEvent.Profile.About = about.String
		}
		if picture.Valid {
			childEvent.Profile.Picture = picture.String
		}

		if website.Valid {
			childEvent.Profile.Website = website.String
		}
		if nip05.Valid {
			childEvent.Profile.Nip05 = nip05.String
		}
		if lud16.Valid {
			childEvent.Profile.Lud16 = lud16.String
		}
		if displayname.Valid {
			childEvent.Profile.DisplayName = displayname.String
		}

		childEvent.Profile.Followed = false
		if followed.Valid {
			childEvent.Profile.Followed = true
		}
		event.Children[childEvent.Event.ID] = childEvent
	}

	return event, err
}

func (st *Storage) FindRawEvent(ctx context.Context, id string) (nostr.Event, error) {
	var qry = `SELECT e.event_id, e.pubkey, e.kind, e.event_created_at, e.content, e.sig, e.tags_full::json
	FROM events e 
	WHERE e.event_id = $1`
	qryStr := qry[:len(qry)-2] + "'" + id + "'"
	log.Println(qryStr)

	var event Event
	event.Event = &nostr.Event{}

	err := st.DbPool.QueryRow(ctx, qry, id).Scan(&event.Event.ID, &event.Event.PubKey, &event.Event.Kind, &event.Event.CreatedAt,
		&event.Event.Content, &event.Event.Sig, &event.Event.Tags)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("404 no event with id %s\n", id)
	case err != nil:
		log.Printf("502 query error: %v\n", err)
	default:
		log.Println("200 Event: ", event)
	}

	return *event.Event, err
}

func (st *Storage) CheckProfiles(ctx context.Context, pubkeys []string, epochtime int64) ([]string, error) {
	qry := `SELECT pubkey FROM profiles WHERE EXTRACT(EPOCH FROM created_at) > $1 AND pubkey in (`

	for _, pubkey := range pubkeys {
		qry = qry + `'` + pubkey + `',`
	}
	qry = qry[:len(qry)-1] + `)`

	log.Println("Ignore these pubkeys for data refreshing: ", qry, epochtime)
	rows, err := st.DbPool.Query(ctx, qry, epochtime)
	if err != nil {
		log.Println(err)
		return []string{}, err
	}

	pubkeysMap := make(map[string]string)
	for _, pk := range pubkeys { // Put all pubkeys in a map
		pubkeysMap[pk] = pk
	}
	log.Println("Map of pubkeys: ", pubkeysMap)
	for rows.Next() {
		var pubkey string
		_ = rows.Scan(&pubkey)
		delete(pubkeysMap, pubkey) // Ignore all pubkeys from the result
	}
	log.Println("Left over pubkeys to get (map): ", pubkeysMap)

	pubkeysFinal := make([]string, 0) // Create empty []string, i think you can also use []string{}
	for _, pk := range pubkeysMap {
		pubkeysFinal = append(pubkeysFinal, pk) // Transform it back to a []string
	}
	log.Println("Left over pubkeys to get (array): ", pubkeysFinal)
	return pubkeysFinal, nil
}

/**
 * Some users just posting garbage, so we try to block those by putting them on the naugthy list
 */
func (st *Storage) BlockPubkey(ctx context.Context, pubkey string) error {
	_, err := st.DbPool.Exec(ctx, `INSERT INTO "block_pubkeys" (pubkey, created_at) VALUES ($1, NOW()) ON CONFLICT (pubkey) DO NOTHING;`, pubkey)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

/**
 * And there are user we like, so put them on the good list
 */
func (st *Storage) FollowPubkey(ctx context.Context, pubkey string) error {
	_, err := st.DbPool.Exec(ctx, `INSERT INTO "follow_pubkeys" (pubkey, created_at) VALUES ($1, NOW()) ON CONFLICT (pubkey) DO NOTHING;`, pubkey)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (st *Storage) UnfollowPubkey(ctx context.Context, pubkey string) error {
	_, err := st.DbPool.Exec(ctx, `DELETE FROM "follow_pubkeys" WHERE pubkey = $1;`, pubkey)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (st *Storage) getLastTimeStamp(ctx context.Context) int64 {
	var createdAt time.Time
	row := st.DbPool.QueryRow(ctx, "SELECT MAX(created_at) as MaxCreated FROM events")
	row.Scan(&createdAt)

	fmt.Println("Timestamp from database: ", createdAt)
	return createdAt.Unix()
}

func (st *Storage) FindProfile(ctx context.Context, pubkey string) (Profile, error) {
	var qry = `SELECT
	name, about, picture, website, nip05, lud16, display_name
	FROM profiles WHERE pubkey = $1`
	//qry = qry + "'" + id + "'"
	log.Println(qry)

	var profile Profile = Profile{}

	var name sql.NullString
	var about sql.NullString
	var picture sql.NullString
	var website sql.NullString
	var nip05 sql.NullString
	var lud16 sql.NullString
	var displayname sql.NullString

	err := st.DbPool.QueryRow(ctx, qry, pubkey).Scan(&name, &about, &picture, &website, &nip05, &lud16, &displayname)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("404 no profile with pubkey %s\n", pubkey)
	case err != nil:
		log.Fatalf("502 query error: %v\n", err)
	}

	if name.Valid {
		profile.Name = name.String
	} else {
		profile.Name = pubkey
	}
	if about.Valid {
		profile.About = about.String
	}
	if picture.Valid {
		profile.Picture = picture.String
	}

	if website.Valid {
		profile.Website = website.String
	}
	if nip05.Valid {
		profile.Nip05 = nip05.String
	}
	if lud16.Valid {
		profile.Lud16 = lud16.String
	}
	if displayname.Valid {
		profile.DisplayName = displayname.String
	}

	return profile, err
}
