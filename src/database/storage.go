package database

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nbd-wtf/go-nostr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

/**
 * We neede an active database connection object.
 * The filter is used for certain words in de posts we want to filter out, because they can be spam
 */
type Storage struct {
	Filter []string
	Count  int64
	GormDB *gorm.DB
}

type DbConfig struct {
	User     string
	Password string
	Dbname   string
	Port     int
	Host     string
}

type UserProfile struct {
	Name        string `json:"name"`
	About       string `json:"about"`
	Picture     string `json:"picture"`
	Website     string `json:"website"`
	Nip05       string `json:"nip05"`
	Lud16       string `json:"lud16"`
	DisplayName string `json:"display_name"`
	Pubkey      string `json:"pubkey"`
	Followed    bool   `json:"followed"`
}

type Event struct {
	Event    *nostr.Event      `json:"event"`
	Profile  UserProfile       `json:"profile"`
	Etags    []string          `json:"etags"`
	Ptags    []string          `json:"ptags"`
	Garbage  bool              `json:"gargabe"`
	Children map[string]*Event `json:"children"`
	Tree     int64             `json:"tree"`
	RootId   string            `json:"root_id"`
	Bookmark bool              `json:"bookmark"`
}

/**
 * Since the above structs should be in sync with the database tables they represent.
 * I put the create statement of the database here even when it more a database thing which is storage.go.
 * Maybe change it later.
 *
 * Payload of pg_notify is 8000. It will crash the app when it is beyond that when using "PERFORM pg_notify('submissions',row_to_json(NEW)::text);"
 * @see https://stackoverflow.com/questions/41057130/postgresql-error-payload-string-too-long
 */
func (st *Storage) CheckError(err error) {
	if err != nil {
		log.Println("Query:: ", err.Error())
		panic(err)
	}
}

func (st *Storage) Paginate(value interface{}, pagination *Pagination, db *gorm.DB) func(db *gorm.DB) *gorm.DB {
	var totalRows int64
	st.GormDB.Model(value).Count(&totalRows)

	pagination.TotalRows = totalRows
	totalPages := int(math.Ceil(float64(totalRows) / float64(pagination.Limit)))
	pagination.TotalPages = totalPages

	return func(db *gorm.DB) *gorm.DB {
		return db.Offset(pagination.GetOffset()).Limit(pagination.GetLimit()).Order(pagination.GetSort())
	}
}

/**
 * Connect to postgresql database
 */
func (st *Storage) Connect(ctx context.Context, cfg *DbConfig) error {
	var err error

	// connection string
	/*
		connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Dbname)

		st.DbPool, err = pgxpool.New(ctx, connStr)
		if err != nil {
			return err
		}
	*/
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Europe/Amsterdam",
		cfg.Host,
		cfg.User,
		cfg.Password,
		cfg.Dbname,
		cfg.Port)

	st.GormDB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

	st.GormDB.Logger.LogMode(logger.Silent)

	st.GormDB.AutoMigrate(&Note{}, &Profile{}, &Block{}, &Follow{}, &Seen{}, &Tree{}, &Bookmark{})

	st.GormDB.Exec(`DO $$ BEGIN
    		CREATE TYPE vote AS ENUM('like','dislike');
 				EXCEPTION
    			WHEN duplicate_object THEN null;
 			END $$;`)
	st.GormDB.AutoMigrate(&Reaction{})
	fmt.Println("Connected to database:", cfg.Dbname)
	return err
}

func (st *Storage) SaveProfile(ctx context.Context, ev *nostr.Event) {
	var data Profile
	err := json.Unmarshal([]byte(ev.Content), &data)
	if err != nil {
		//log.Println("Query:: ", err.Error(), ev.Content)
		return
	}

	profile := Profile{
		Pubkey:      ev.PubKey,
		Name:        data.Name,
		About:       data.About,
		Picture:     data.Picture,
		Website:     data.Website,
		Nip05:       data.Nip05,
		Lud16:       data.Lud16,
		DisplayName: data.DisplayName,
		Raw:         ev.String()}
	//st.GormDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&profile)
	st.GormDB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pubkey"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "about", "picture", "website", "nip05", "lud16", "display_name", "raw"}),
	}).Create(&profile)

	if err := st.GormDB.WithContext(ctx).Exec("UPDATE notes SET profile_id=?,updated_at=NOW() WHERE pubkey =?", profile.ID, ev.PubKey).Error; err != nil {
		fmt.Println(err)
	}
	/*if err := st.GormDB.WithContext(ctx).Model(&Note{}).Where("pubkey = ?", ev.PubKey).Update("profile_id", profile.ID).Error; err != nil {
		fmt.Println(err)
	}*/

	// Should be in a dynamic list, so you can add to it or remove items.
	if data.Picture != "" && len(data.Picture) > len("https://randomuser.me") && data.Picture[0:len("https://randomuser.me")] == "https://randomuser.me" {
		st.CreateBlock(ctx, ev.PubKey)
	}
}

/**
 * Save user profiles for easy lookup
 */
func (st *Storage) SaveProfiles(ctx context.Context, evs []*nostr.Event) {
	for _, ev := range evs {
		st.SaveProfile(ctx, ev)
	}
}

/**
 * Save the events, mostly notes. Ignore duplicate events based on unique event id
 * This will normalize the content tag of the events with all the unwanted markup (Myaby put this in a helper function)
 */
func (st *Storage) SaveEvents(ctx context.Context, evs []*nostr.Event) []string {
	var pubkeys = make([]string, 0)
	ptags, etags := make([]string, 0), make([]string, 0)

	type EventTree struct {
		RootTag  string
		ReplyTag string
	}

	//var re = regexp.MustCompile(`@npub`)

	var tree EventTree
	for _, ev := range evs {
		if ev.CreatedAt.Time().Unix() > time.Now().Unix() {
			//fmt.Fprintf(os.Stderr, "log message: %s", "QUERY:: Ignoring this event because timestamp is in the future."+ev.String())
			continue
		}

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
					log.Println("Query:: P# tag not valid: ", tag)
					continue
				} else {
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
			}
		}
		if strings.Count(ev.Content, "@npub") > 4 {
			Garbage = true
		}
		//re.MatchString(ev.Content)

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

		note, _ := st.SaveNote(ctx, ev, tagJson, ptags, etags, Garbage, jsonbuf.Bytes())

		if note.ID > 0 && len(tree.RootTag) > 0 {
			treeData := Tree{EventId: ev.ID, RootEventId: tree.RootTag, ReplyEventId: tree.ReplyTag}
			st.GormDB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&treeData)
		}

		if note.ID > 0 && ev.Kind == 0 {
			st.SaveProfile(ctx, ev)
		}

		// votes
		if note.ID > 0 && ev.Kind == 7 && len(etags) > 0 {
			t := ev.Tags.GetFirst([]string{"e"})
			if t != nil {
				targetEventId := t.Value()

				var result Note
				st.GormDB.WithContext(ctx).Where("event_id = ?", targetEventId).Find(&result) // only add votes for existing

				if result.ID > 0 {
					st.SaveReaction(ctx, ev, targetEventId, note.ID)
				}
			}
		}
	}

	return pubkeys
}

func (st *Storage) SaveNote(ctx context.Context, ev *nostr.Event, tagJson []byte, ptags []string, etags []string, Garbage bool, json []byte) (Note, error) {
	note := Note{}
	note.EventId = ev.ID
	note.Pubkey = ev.PubKey
	note.Kind = ev.Kind
	note.EventCreatedAt = ev.CreatedAt.Time().Unix()
	note.Content = ev.Content
	note.TagsFull = string(tagJson)
	note.Sig = ev.Sig
	note.Ptags = ptags
	note.Etags = etags
	note.Garbage = Garbage
	note.Raw = json

	err := st.GormDB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&note).Error

	if err != nil {
		return Note{}, err
	}

	return note, nil
}

func (st *Storage) SaveReaction(ctx context.Context, ev *nostr.Event, targetEventId string, noteId uint) {
	vote := Reaction{
		Pubkey:        ev.PubKey,
		Content:       ev.Content,
		CurrentVote:   Like,
		TargetEventId: targetEventId,
		FromEventId:   ev.ID,
		NoteID:        noteId}
	if ev.Content == "-" {
		vote.CurrentVote = Dislike
	}

	err := st.GormDB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&vote).Error
	if err != nil {
		log.Println("Query error:: ", err.Error())
		panic(err)
	}
}

/**
 * Get a limitted amount of stored events
 */
func (st *Storage) GetEvents(ctx context.Context, limit int) (*[]Event, error) {
	rows, err := st.GormDB.WithContext(ctx).Raw(`SELECT e.event_id, e.pubkey, e.kind, e.event_created_at, e.content, e.tags_full::json,e.sig, e.etags, e.ptags, u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name FROM notes e 
	LEFT JOIN profiles u ON (u.pubkey = e.pubkey ) 
	LEFT JOIN blocks b on (b.pubkey = e.pubkey) 
	LEFT JOIN seens s on (s.event_id = e.event_id)
	WHERE e.kind = 1 AND b.pubkey IS NULL AND s.event_id IS NULL AND e.garbage = false ORDER BY e.event_created_at DESC LIMIT $1`, limit).Rows()

	if err != nil {
		log.Println("Query:: ", err)
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
			log.Println("Query:: ", err.Error())
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
		log.Println("Query:: ", err)
		return nil, err
	}

	return &events, nil
}

type Options struct {
	Follow   bool
	BookMark bool
}

/**
 * Do not show all data in an endless scrol page, but paginate it for easy access
 * and ignore the garbage tagged posts
 *
 */
func (st *Storage) GetEventPagination(ctx context.Context, p *Pagination, options Options) error {

	tx := st.GormDB.Model(&Note{}).
		Select(`notes.id, notes.event_id, notes.pubkey, notes.kind, notes.event_created_at, 
		notes.content, notes.tags_full::json, notes.sig, notes.etags, notes.ptags,
		profiles.name, profiles.about , profiles.picture,
		profiles.website, profiles.nip05, profiles.lud16, profiles.display_name, 
		follows.pubkey follow, bookmarks.event_id bookmarked`).
		Joins("LEFT JOIN profiles ON (profiles.pubkey = notes.pubkey)").
		Joins("LEFT JOIN blocks  on (blocks.pubkey = notes.pubkey)").
		Joins("LEFT JOIN seens on (seens.event_id = notes.event_id)").
		Where("notes.kind = 1").
		Where("notes.etags='{}'").
		Where("blocks.pubkey IS NULL").
		Where("seens.event_id IS NULL").
		Where("notes.garbage = false")

	if p.Since > 0 && (!options.BookMark && !options.Follow) {
		since := time.Now().Unix() - int64(p.Since*60*60*24)
		tx.Where("notes.event_created_at > ?", fmt.Sprintf("%d", since))
	}

	if options.Follow {
		tx.Joins("JOIN follows ON (follows.pubkey = notes.pubkey)")
	} else {
		tx.Joins("LEFT JOIN follows ON (follows.pubkey = notes.pubkey)")
	}

	if options.BookMark {
		tx.Joins("JOIN bookmarks ON (bookmarks.event_id = notes.event_id)")
	} else {
		tx.Joins("LEFT JOIN bookmarks ON (bookmarks.event_id = notes.event_id)")
	}

	if p.MaxId > 0 && !p.Renew && !options.Follow && !options.BookMark {
		tx.Where("notes.id <= ?", fmt.Sprintf("%d", p.MaxId))
	}

	var count int64
	tx.Count(&count)

	tx.Limit(p.Limit)
	if p.Offset > 0 {
		tx.Offset(int(p.Offset))
	}

	tx.Order("notes.event_created_at DESC")
	rows, err := tx.Rows()
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	p.SetTotal(count)
	p.SetTo()
	eventMap, keys, _ := st.procesEventRows(rows)

	st.getChildren(ctx, eventMap)
	events := make([]Event, 0)
	// Make sure the order stays the same
	for _, k := range keys {
		events = append(events, eventMap[k])
	}

	p.Data = events
	return nil
}

func (st *Storage) procesEventRows(rows *sql.Rows) (map[string]Event, []string, error) {
	eventMap := make(map[string]Event)
	var keys []string

	for rows.Next() {
		var note Event
		var id int
		var name sql.NullString
		var about sql.NullString
		var picture sql.NullString

		var website sql.NullString
		var nip05 sql.NullString
		var lud16 sql.NullString
		var displayname sql.NullString
		var followed sql.NullString
		var bookmarked sql.NullString

		note.Event = &nostr.Event{}
		if err := rows.Scan(&id,
			&note.Event.ID, &note.Event.PubKey,
			&note.Event.Kind, &note.Event.CreatedAt,
			&note.Event.Content, &note.Event.Tags, &note.Event.Sig,
			(pq.Array)(&note.Etags), (pq.Array)(&note.Ptags),
			&name, &about, &picture, &website, &nip05, &lud16, &displayname, &followed, &bookmarked); err != nil {
			log.Println("Query:: ", err.Error())
			panic(err)
		}

		if _, ok := eventMap[note.Event.ID]; ok {
			continue
		}

		note.RootId = note.Event.ID
		note.Tree = 1

		if name.Valid {
			note.Profile.Name = name.String
		} else {
			note.Profile.Name = note.Event.PubKey
		}
		if about.Valid {
			note.Profile.About = about.String
		}
		if picture.Valid {
			note.Profile.Picture = picture.String
		}

		if website.Valid {
			note.Profile.Website = website.String
		}
		if nip05.Valid {
			note.Profile.Nip05 = nip05.String
		}
		if lud16.Valid {
			note.Profile.Lud16 = lud16.String
		}
		if displayname.Valid {
			note.Profile.DisplayName = displayname.String
		}

		note.Profile.Followed = false
		if followed.Valid {
			note.Profile.Followed = true
		}
		note.Bookmark = false
		if bookmarked.Valid {
			note.Bookmark = true
		}

		//nostr.Event = json.Unmarshal()
		//sdk.ParseReferences(&nostr.Event{event})

		note.Children = make(map[string]*Event, 0)
		eventMap[note.Event.ID] = note
		keys = append(keys, note.Event.ID) // Make sure the order stays the same @see https://go.dev/blog/maps
	}
	// Check for errors from iterating over rows.
	if err := rows.Err(); err != nil {
		log.Println("Query:: ", err)
		return nil, nil, err
	}
	defer rows.Close()

	return eventMap, keys, nil
}

func (st *Storage) getChildren(ctx context.Context, eventMap map[string]Event) error {
	var err error

	/**
	 * Get all child notes
	 */

	tx := st.GormDB.Model(&Note{}).
		Select(`trees.root_event_id, trees.reply_event_id, notes.id, notes.event_id, notes.pubkey, notes.kind, notes.event_created_at, 
		notes.content, notes.tags_full::json, notes.sig, notes.etags, notes.ptags,
		profiles.name, profiles.about , profiles.picture,
		profiles.website, profiles.nip05, profiles.lud16, profiles.display_name, 
		follows.pubkey follow, bookmarks.event_id bookmarked`).
		Joins("JOIN trees ON (trees.event_id = notes.event_id)").
		Joins("LEFT JOIN profiles ON (profiles.pubkey = notes.pubkey)").
		Joins("LEFT JOIN blocks ON (blocks.pubkey = notes.pubkey)").
		Joins("LEFT JOIN seens on (seens.event_id = notes.event_id)").
		Joins("LEFT JOIN follows ON (follows.pubkey = notes.pubkey)").
		Joins("LEFT JOIN bookmarks ON (bookmarks.event_id = notes.event_id)").
		Where("notes.kind = 1").
		Where("notes.etags='{}'").
		Where("blocks.pubkey IS NULL").
		Where("seens.event_id IS NULL").
		Where("notes.garbage = false")

	var eventIds string
	var numIds int = 0
	for k := range eventMap {
		eventIds = eventIds + `'` + k + `',`
		numIds++
	}
	if numIds < 1 {
		return nil
	}

	eventIds = eventIds[:len(eventIds)-1]

	tx.Where("trees.root_event_id IN (" + eventIds + ")")

	treeRows, err := tx.Rows()
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
		var bookmarked sql.NullString
		childEvent.Event = &nostr.Event{}

		if err := treeRows.Scan(&root_event_id, &reply_event_id, &id,
			&childEvent.Event.ID, &childEvent.Event.PubKey, &childEvent.Event.Kind, &childEvent.Event.CreatedAt, &childEvent.Event.Content, &childEvent.Event.Tags, &childEvent.Event.Sig,
			(pq.Array)(&childEvent.Etags), (pq.Array)(&childEvent.Ptags), &name, &about, &picture,
			&website, &nip05, &lud16, &displayname, &followed, &bookmarked); err != nil {
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
		childEvent.Bookmark = false
		if bookmarked.Valid {
			childEvent.Bookmark = true
		}

		if item, ok := eventMap[root_event_id]; ok {

			if reply_event_id.Valid {
				if reply_event_id.String == "" {
					childEvent.Tree = 2
					childEvent.Children = make(map[string]*Event, 0)
					item.Children[childEvent.Event.ID] = &childEvent
					//eventMap[root_event_id] = item
				}
				if reply_event_id.String != "" {
					//fmt.Println("Child [", childEvent.Event.ID, "] :: replied to ["+reply_event_id.String+"] "+childEvent.Event.Content)
					walk(&item, childEvent, reply_event_id.String, 1)
				}
			}

		}
	}
	if err := treeRows.Err(); err != nil {
		log.Println(err)
	}
	treeRows.Close()

	return nil
}

func walk(parent *Event, payload Event, reply_event_id string, level int64) bool {
	if level > 8 {
		return false
	}

	if parent.Event.ID == reply_event_id {
		//fmt.Println("Found parent:: [", parent.Event.ID, "] :: ", parent.Event.Content)
		payload.Tree = level
		if parent.Children == nil {
			parent.Children = make(map[string]*Event, 0)
		}
		parent.Children[payload.Event.ID] = &payload
		return true
	}

	if len(parent.Children) > 0 {
		for _, Node := range parent.Children {
			if ok := walk(Node, payload, reply_event_id, level+1); ok {
				return true
			}
		}
	}
	return false
}

func (st *Storage) GetInbox(ctx context.Context, p *Pagination, pubkey string) error {
	qry := `
	SELECT
	e0.id, e0.event_id, e0.pubkey, e0.kind, e0.event_created_at, e0.content, e0.tags_full::json, e0.sig, e0.etags, e0.ptags,  
	u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name, f.pubkey follow, bm.event_id bookmarked
	FROM 
	notes e0 
	LEFT JOIN follows f ON (f.pubkey = e0.pubkey)
	LEFT JOIN bookmarks bm ON (bm.event_id = e0.event_id)
	LEFT JOIN profiles u ON (u.pubkey = e0.pubkey ) 
	JOIN
	(SELECT
	DISTINCT e0.event_id
	FROM 
	notes e0 
	JOIN (SELECT t.root_event_id, t.event_id, t.reply_event_id FROM trees t, notes e1 WHERE e1.pubkey = $1 AND e1.event_id = t.event_id) t0 ON e0.event_id = t0.root_event_id
	) tbl 
	ON
	tbl.event_id = e0.event_id
	ORDER BY e0.event_created_at DESC;`

	//rows, err := tx.Query(ctx, qry, pubkey)
	rows, err := st.GormDB.WithContext(ctx).Raw(qry, pubkey).Rows()
	if err != nil {
		log.Println("Query:: ", err)
		return nil
	}
	defer rows.Close()

	eventMap, keys, _ := st.procesEventRows(rows)
	p.SetTotal(int64(len(keys)))
	p.SetTo()

	st.getChildren(ctx, eventMap)
	events := make([]Event, 0)
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
	FROM notes e LEFT JOIN profiles u ON (u.pubkey = e.pubkey ) 
	LEFT JOIN blocks b on (b.pubkey = e.pubkey) 
	WHERE e.event_id = $1`
	//qry = qry + "'" + id + "'"
	log.Println("Query:: ", qry)

	var event Event
	var name sql.NullString
	var about sql.NullString
	var picture sql.NullString

	var website sql.NullString
	var nip05 sql.NullString
	var lud16 sql.NullString
	var displayname sql.NullString
	event.Event = &nostr.Event{}

	row := st.GormDB.WithContext(ctx).Raw(qry, id).Row()
	err := row.Scan(&event.Event.ID, &event.Event.PubKey, &event.Event.Kind, &event.Event.CreatedAt, &event.Event.Content, &event.Event.Tags, &event.Event.Sig,
		(pq.Array)(&event.Etags), (pq.Array)(&event.Ptags), &name, &about, &picture, &website, &nip05, &lud16, &displayname)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("Query:: 404 no event with id %s\n", id)
	case err != nil:
		log.Fatalf("Query:: 502 query error: %v\n", err)
	}
	event.Tree = 1
	event.RootId = event.Event.ID
	event.Children = make(map[string]*Event, 0)

	treeQry := `SELECT t.root_event_id, t.reply_event_id, 
	e.id, e.event_id, e.pubkey, e.kind, e.event_created_at, e.content,e.tags_full::json,e.sig, 
	e.etags, e.ptags , u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name, f.pubkey follow
	FROM trees t, notes e 
	LEFT JOIN profiles u ON (u.pubkey = e.pubkey ) 
	LEFT JOIN blocks b on (b.pubkey = e.pubkey) 
	LEFT JOIN seens s on (s.event_id = e.event_id)
	LEFT JOIN follows f ON (f.pubkey = e.pubkey)
	WHERE root_event_id IN (` + `'` + event.Event.ID + `')` +
		` AND e.event_id = t.event_id
	AND e.kind = 1 AND b.pubkey IS NULL AND s.event_id IS NULL AND e.garbage = false;`

	var treeRows *sql.Rows
	treeRows, err = st.GormDB.WithContext(ctx).Raw(treeQry).Rows()
	if err != nil {
		log.Println("Query:: ", err)
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
			(pq.Array)(&childEvent.Etags), (pq.Array)(&childEvent.Ptags), &name, &about, &picture,
			&website, &nip05, &lud16, &displayname, &followed); err != nil {
			log.Println("Query:: ", err.Error())
			panic(err)
		}

		childEvent.RootId = event.Event.ID
		childEvent.Tree = 2
		childEvent.Children = make(map[string]*Event, 0)

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

		event.Children[childEvent.Event.ID] = &childEvent
	}

	return event, err
}

func (st *Storage) FindRawEvent(ctx context.Context, id string) (nostr.Event, error) {
	var qry = `SELECT e.event_id, e.pubkey, e.kind, e.event_created_at, e.content, e.sig, e.tags_full::json
	FROM notes e 
	WHERE e.event_id = $1`

	var event Event
	event.Event = &nostr.Event{}

	row := st.GormDB.WithContext(ctx).Raw(qry, id).Row()

	err := row.Scan(&event.Event.ID, &event.Event.PubKey, &event.Event.Kind, &event.Event.CreatedAt,
		&event.Event.Content, &event.Event.Sig, &event.Event.Tags)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("Query:: 404 no event with id %s\n", id)
	case err != nil:
		log.Printf("Query:: 502 query error: %v\n", err)
	}

	return *event.Event, err
}

func (st *Storage) CheckProfiles(ctx context.Context, pubkeys []string, epochtime int64) ([]string, error) {
	qry := `SELECT pubkey FROM profiles WHERE EXTRACT(EPOCH FROM created_at) > $1 AND pubkey in (`

	for _, pubkey := range pubkeys {
		qry = qry + `'` + pubkey + `',`
	}
	qry = qry[:len(qry)-1] + `)`

	rows, err := st.GormDB.WithContext(ctx).Raw(qry, epochtime).Rows()
	if err != nil {
		log.Println("Query:: ", err)
		return []string{}, err
	}

	pubkeysMap := make(map[string]string)
	for _, pk := range pubkeys { // Put all pubkeys in a map
		pubkeysMap[pk] = pk
	}
	for rows.Next() {
		var pubkey string
		_ = rows.Scan(&pubkey)
		delete(pubkeysMap, pubkey) // Ignore all pubkeys from the result
	}

	pubkeysFinal := make([]string, 0) // Create empty []string, i think you can also use []string{}
	for _, pk := range pubkeysMap {
		pubkeysFinal = append(pubkeysFinal, pk) // Transform it back to a []string
	}

	return pubkeysFinal, nil
}

/**
 * Some users just posting garbage, so we try to block those by putting them on the naugthy list
 */
func (st *Storage) CreateBlock(ctx context.Context, pubkey string) error {
	blockPubkey := Block{Pubkey: pubkey}
	//st.GormDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&blockPubkey)

	if err := st.GormDB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&blockPubkey).Error; err != nil {
		fmt.Println(err)
	}

	if err := st.GormDB.WithContext(ctx).Model(&Note{}).Where("pubkey = ?", pubkey).Update("block_id", blockPubkey.ID).Error; err != nil {
		fmt.Println(err)
	}

	return nil
}

/**
 * And there are user we like, so put them on the good list
 */
func (st *Storage) CreateFollow(ctx context.Context, pubkey string) error {
	followPubkey := Follow{Pubkey: pubkey}
	if err := st.GormDB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&followPubkey).Error; err != nil {
		fmt.Println(err)
	}

	if err := st.GormDB.WithContext(ctx).Model(&Note{}).Where("pubkey = ?", pubkey).Update("follow_id", followPubkey.ID).Error; err != nil {
		fmt.Println(err)
	}

	return nil
}

func (st *Storage) RemoveFollow(ctx context.Context, pubkey string) error {
	if err := st.GormDB.WithContext(ctx).Where("pubkey = ?", pubkey).Delete(&Follow{}); err != nil {
		fmt.Println(err)
	}

	if err := st.GormDB.WithContext(ctx).Model(&Note{}).Where("pubkey = ?", pubkey).Update("follow_id", gorm.Expr("NULL")); err != nil {
		fmt.Println(err)
	}

	return nil
}

func (st *Storage) CreateBookMark(ctx context.Context, eventID string) error {
	var note Note
	st.GormDB.WithContext(ctx).Where("event_id = ?", eventID).Find(&note)

	bookmark := Bookmark{EventId: eventID, NoteID: note.ID}
	err := st.GormDB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&bookmark).Error
	if err != nil {
		log.Println("Query:: ", err)
		return err
	}

	return nil
}

func (st *Storage) RemoveBookMark(ctx context.Context, eventID string) error {
	err := st.GormDB.WithContext(ctx).Where("event_id = ?", eventID).Delete(&Bookmark{}).Error
	if err != nil {
		log.Println("Query:: ", err)
		return err
	}

	return nil
}

func (st *Storage) GetLastTimeStamp(ctx context.Context) int64 {
	var createdAt time.Time
	st.GormDB.WithContext(ctx).Raw("SELECT MAX(created_at) as MaxCreated FROM notes").Scan(&createdAt)

	return createdAt.Unix()
}

func (st *Storage) FindProfile(ctx context.Context, pubkey string) (Profile, error) {
	var qry = `SELECT
	name, about, picture, website, nip05, lud16, display_name
	FROM profiles WHERE pubkey = $1`

	var profile Profile = Profile{}

	var name sql.NullString
	var about sql.NullString
	var picture sql.NullString
	var website sql.NullString
	var nip05 sql.NullString
	var lud16 sql.NullString
	var displayname sql.NullString

	row := st.GormDB.WithContext(ctx).Raw(qry, pubkey).Row()
	err := row.Scan(&name, &about, &picture, &website, &nip05, &lud16, &displayname)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("Query:: 404 no profile with pubkey %s\n", pubkey)
	case err != nil:
		log.Fatalf("Query:: 502 query error: %v\n", err)
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
