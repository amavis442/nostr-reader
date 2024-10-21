package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nbd-wtf/go-nostr"
	sdk "github.com/nbd-wtf/nostr-sdk"
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
	Filter        []string
	Count         int64
	GormDB        *gorm.DB
	Env           string
	Pubkey        string
	Notifications []string
}

type DbConfig struct {
	User      string
	Password  string
	Dbname    string
	Port      int
	Host      string
	Retention int
}

var Missing_event_ids []string

func (st *Storage) SetEnvironment(env string) {
	st.Env = env
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

/**
 * Connect to postgresql database
 */
func (st *Storage) Connect(ctx context.Context, cfg *DbConfig) error {
	var err error

	st.Notifications = make([]string, 0)

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Europe/Amsterdam",
		cfg.Host,
		cfg.User,
		cfg.Password,
		cfg.Dbname,
		cfg.Port)

	st.GormDB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:      logger.Default.LogMode(logger.Silent),
		PrepareStmt: true,
	})

	st.GormDB.AutoMigrate(&Note{}, &Profile{}, &Notification{}, &Block{}, &Follow{}, &Seen{}, &Tree{}, &Bookmark{}, &Relay{})

	st.GormDB.Exec(`DO $$ BEGIN
		    		CREATE TYPE vote AS ENUM('like','dislike');
		 				EXCEPTION
		    			WHEN duplicate_object THEN null;
		 			END $$;`)
	st.GormDB.AutoMigrate(&Reaction{})

	st.GormDB.Exec(`CREATE OR REPLACE FUNCTION delete_submission() RETURNS trigger AS $$
			BEGIN
		  		IF NEW.kind=5 THEN
		       		DELETE FROM notes WHERE ARRAY[event_id] && NEW.etags AND NEW.pubkey=pubkey;
		    		RETURN NULL;
		  		END IF;
		  		RETURN NEW;
			END;
			$$ LANGUAGE plpgsql;`)
	st.GormDB.Exec(`DROP TRIGGER IF EXISTS delete_trigger ON notes;`)
	st.GormDB.Exec(`CREATE TRIGGER delete_trigger BEFORE INSERT ON notes FOR EACH ROW EXECUTE FUNCTION delete_submission();`)

	log.Printf(`Connect() -> Cleaning history older then %d days`, cfg.Retention)
	then := time.Now().AddDate(0, 0, -1*cfg.Retention)
	past := fmt.Sprintf("%d-%d-%d 00:00:00\n",
		then.Year(),
		then.Month(),
		then.Day())

	st.GormDB.Transaction(func(tx *gorm.DB) error {
		err = tx.Exec(`DELETE FROM reactions WHERE note_id in (SELECT id FROM notes WHERE created_at <= $1);`, past).Error
		if err != nil {
			log.Println(err.Error())
			return err
		}
		err = tx.Exec(`DELETE FROM notifications WHERE note_id in (SELECT id FROM notes WHERE created_at <= $1);`, past).Error
		if err != nil {
			log.Println(err.Error())
			return err
		}
		err = tx.Exec(`DELETE FROM notes WHERE created_at <= $1;`, past).Error
		if err != nil {
			log.Println(err.Error())
			return err
		}

		return nil
	})
	Missing_event_ids = make([]string, 0)

	fmt.Println("Connect() -> Connected to database:", cfg.Dbname)
	return err
}

func (st *Storage) SaveProfile(ctx context.Context, ev *Event) error {
	var data Profile
	content := ev.Event.Content
	err := json.Unmarshal([]byte(content), &data)
	if err != nil {
		//log.Println("Query:: ", err.Error(), ev.Content)
		return err
	}

	jsonbuf := bytes.NewBuffer(nil)
	jsonbuf.Reset()
	enc := json.NewEncoder(jsonbuf)
	// turn off stupid go json encoding automatically doing HTML escaping...
	enc.SetEscapeHTML(false)
	if err := enc.Encode(ev.Event); err != nil {
		log.Println(err)
		return err
	}

	profile := Profile{
		Pubkey:      ev.Event.PubKey,
		Name:        data.Name,
		About:       data.About,
		Picture:     data.Picture,
		Website:     data.Website,
		Nip05:       data.Nip05,
		Lud16:       data.Lud16,
		DisplayName: data.DisplayName,
		Raw:         jsonbuf.Bytes(),
		Urls:        ev.Urls,
	}
	profile.UpdatedAt.Time = time.Now()

	slog.Info("SaveProfile() -> Adding or updating profile: pubkey = "+profile.Pubkey, "profile", profile)

	//log.Println("SaveProfile() -> Adding or updating profile: pubkey = ", profile.Pubkey, " [", profile, "]")
	log.Println("----------------------")
	//st.GormDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&profile)
	/*
		st.GormDB.Model(&Profile{}).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "pubkey"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "about", "picture", "website", "nip05", "lud16", "display_name", "raw", "urls", "updated_at"}),
		}).Create(&profile)
	*/
	var searchProfile Profile
	if err := st.GormDB.Model(&Profile{}).Where(Profile{Pubkey: ev.Event.PubKey}).Find(&searchProfile).Error; err != nil {
		switch {
		case err == sql.ErrNoRows:
			log.Printf("FindProfile() -> Query:: 404 no profile with pubkey %s\n", ev.Event.PubKey)
		case err != nil:
			log.Fatalf("FindProfile() -> Query:: 502 query error: %v\n", err)
		}
		return err
	}

	result := st.GormDB.Where(Profile{Pubkey: ev.Event.PubKey}).
		Assign(profile).
		FirstOrCreate(&profile)
	if result.Error != nil {
		return result.Error
	}
	fmt.Println(profile)

	if st.GormDB.Error != nil {
		log.Print(st.GormDB.Error.Error())
		return st.GormDB.Error
	}

	// Should be in a dynamic list, so you can add to it or remove items.
	if data.Picture != "" && len(data.Picture) > len("https://randomuser.me") && data.Picture[0:len("https://randomuser.me")] == "https://randomuser.me" {
		st.CreateBlock(ctx, ev.Event.PubKey)
	}
	if searchProfile.ID != 0 {
		st.GormDB.Model(&Note{}).Where(Note{Pubkey: ev.Event.PubKey}).Update("profile_id", searchProfile.ID)
	}
	return nil
}

/**
 * Save user profiles for easy lookup
 */
func (st *Storage) SaveProfiles(ctx context.Context, evs []*Event) error {
	for _, ev := range evs {
		err := st.SaveProfile(ctx, ev)
		if err != nil {
			return err
		}
	}

	return nil
}

/**
 * Save the events, mostly notes. Ignore duplicate events based on unique event id
 * This will normalize the content tag of the events with all the unwanted markup (Myaby put this in a helper function)
 */
func (st *Storage) SaveEvents(ctx context.Context, evs []*Event) ([]string, error) {
	var pubkeys = make([]string, 0)

	st.Notifications = make([]string, 0)  // reset if already set
	Missing_event_ids = make([]string, 0) //reset

	for _, ev := range evs {
		if ev.Event.CreatedAt.Time().Unix() > time.Now().Unix() { // Ignore events with timestamp in the future.
			continue
		}

		etags, _, _, _, _ := ProcessTags(ev.Event, st.Pubkey)

		if ev.Event.Kind == 0 {
			err := st.SaveProfile(ctx, ev)
			if err != nil {
				return []string{}, err
			}
		}

		if ev.Event.Kind == 1 {

			note, err := st.SaveNote(ctx, ev)
			if err != nil {
				return []string{}, err
			}
			pubkeys = append(pubkeys, note.Pubkey)
		}

		// votes
		if ev.Event.Kind == 7 && len(etags) > 0 {
			t := ev.Event.Tags.GetFirst([]string{"e"})
			if t != nil {
				targetEventId := t.Value()

				var result Note
				st.GormDB.WithContext(ctx).Where("event_id = ?", targetEventId).Find(&result) // only add votes for existing

				if result.ID > 0 {
					st.SaveReaction(ctx, ev.Event, targetEventId, result.ID)
				}
			}
		}
	}
	return pubkeys, nil
}

func (st *Storage) SaveNote(ctx context.Context, event *Event) (Note, error) {
	var tree EventTree
	ev := event.Event
	etags, ptags, hasNotification, isRoot, tree := ProcessTags(ev, st.Pubkey)
	ptagsNum := len(ptags)
	etagsNum := len(etags)

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
		return Note{}, err
	}

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
	note.Raw = jsonbuf.Bytes()
	note.Root = isRoot
	note.Urls = event.Urls
	note.UpdatedAt.Time = time.Now()

	var searchProfile Profile
	if err := st.GormDB.Model(&Profile{}).Where(Profile{Pubkey: ev.PubKey}).Find(&searchProfile).Error; err != nil {
		switch {
		case err == sql.ErrNoRows:
			log.Printf("FindProfile() -> Query:: 404 no profile with pubkey %s\n", ev.PubKey)
		default:
			log.Fatalf("FindProfile() -> Query:: 502 query error: %v\n", err)
		}
		return Note{}, err
	}
	if searchProfile.ID > 0 {
		note.ProfileID = &searchProfile.ID
	}

	tx := st.GormDB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true})
	if st.Env == "devel" {
		tx = tx.Debug()
	}

	//if !hasNotification {
	err = tx.Create(&note).Error
	//}

	if err != nil {
		return Note{}, err
	}

	if note.ID > 0 && len(tree.RootTag) > 0 {
		var treeSearch Tree
		if len(tree.ReplyTag) > 0 {
			err = st.GormDB.Model(&Tree{}).Where(&Tree{RootEventId: tree.RootTag, ReplyEventId: tree.ReplyTag}).Find(&treeSearch).Error
		} else {
			err = st.GormDB.Model(&Tree{}).Where(&Tree{RootEventId: tree.RootTag}).Find(&treeSearch).Error
		}
		if err != nil {
			return Note{}, err
		}
		if treeSearch.ID == 0 {
			treeData := Tree{EventId: ev.ID, RootEventId: tree.RootTag, ReplyEventId: tree.ReplyTag}
			st.GormDB.Model(&Tree{}).WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&treeData)
		}
		if hasNotification {
			notification := Notification{NoteID: note.ID}
			st.GormDB.Model(&Notification{}).WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&notification)
		}

		// Check if we already have the root Note
		var searchNoteRootNote Note
		err = st.GormDB.Model(&Note{}).Where(&Note{EventId: tree.RootTag}).Find(&searchNoteRootNote).Error
		if err != nil {
			return Note{}, err
		}
		if searchNoteRootNote.ID == 0 {
			Missing_event_ids = append(Missing_event_ids, tree.RootTag)
		}
		// Same goes for reply which is replied to
		if len(tree.ReplyTag) > 0 {
			var searchNoteReplyNote Note
			err = st.GormDB.Model(&Note{}).Where(&Note{EventId: tree.ReplyTag}).Find(&searchNoteReplyNote).Error
			if err != nil {
				return Note{}, err
			}
			if searchNoteReplyNote.ID == 0 {
				Missing_event_ids = append(Missing_event_ids, tree.ReplyTag)
			}
		}
	}

	return note, nil
}

func (st *Storage) SaveReaction(ctx context.Context, ev *nostr.Event, targetEventId string, notesId uint) {
	vote := Reaction{
		Pubkey:        ev.PubKey,
		Content:       ev.Content,
		CurrentVote:   Like,
		TargetEventId: targetEventId,
		FromEventId:   ev.ID,
		NoteID:        notesId,
	}
	if ev.Content == "-" {
		vote.CurrentVote = Dislike
	}

	err := st.GormDB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&vote).Error
	if err != nil {
		log.Println("SaveReaction() -> Query error:: ", err.Error())
		panic(err)
	}
}

type Options struct {
	Follow   bool
	BookMark bool
}

func (st *Storage) GetNewNotesCount(ctx context.Context, maxId int, options Options) (int, error) {
	var count int
	tx := st.GormDB.Model(&Note{}).
		Select(`COUNT(notes.id)`).
		Joins("LEFT JOIN blocks  on (blocks.pubkey = notes.pubkey)").
		Where("notes.kind = 1").
		Where("notes.root = true").
		Where("blocks.pubkey IS NULL").
		Where("notes.garbage = false").
		Where("notes.id > ?", maxId)

	if options.Follow {
		tx.Joins("JOIN follows ON (follows.pubkey = notes.pubkey)")
	} else {
		tx.Joins("LEFT JOIN follows ON (follows.pubkey = notes.pubkey)")
		tx.Where("follows.pubkey is null")
	}

	if options.BookMark {
		tx.Joins("JOIN bookmarks ON (bookmarks.event_id = notes.event_id)")
	}

	tx.Scan(&count)

	if tx.Error != nil {
		return 0, tx.Error
	}
	return count, nil
}

func (st *Storage) GetLastSeenID(ctx context.Context) (int, error) {
	var maxId int
	tx := st.GormDB.Model(&Seen{}).
		Select(`MAX(coalesce(note_id,0))`).Scan(&maxId)

	if tx.Error != nil {
		return 0, tx.Error
	}
	return maxId, nil
}

/**
 * Do not show all data in an endless scrol page, but paginate it for easy access
 * and ignore the garbage tagged posts
 *
 */
func (st *Storage) GetPagination(ctx context.Context, p *Pagination, options Options) error {

	tx := st.GormDB.Debug().Table("notes").
		Select(`notes.id, notes.event_id, notes.pubkey, notes.kind, notes.event_created_at, 
		notes.content, notes.tags_full::json, notes.sig, notes.etags, notes.ptags,
		profiles.name, profiles.about , profiles.picture,
		profiles.website, profiles.nip05, profiles.lud16, profiles.display_name, 
		CASE WHEN length(follows.pubkey) > 0 THEN TRUE ELSE FALSE END followed, 
		CASE WHEN length(bookmarks.event_id) > 0 THEN TRUE ELSE FALSE END bookmarked`).
		Joins("LEFT JOIN profiles ON (profiles.pubkey = notes.pubkey)").
		Joins("LEFT JOIN blocks  on (blocks.pubkey = notes.pubkey)").
		//Joins("LEFT JOIN seens on (seens.event_id = notes.event_id)").
		Where("notes.kind = 1").
		//Where("seens.event_id IS NULL").
		Where("notes.garbage = false")

	if !options.BookMark {
		tx.Where("notes.root = true").
			Where("blocks.pubkey IS NULL")
	}

	// Needs a time stamp that stays the same else total will change with every call
	if p.Since > 0 && (!options.BookMark && !options.Follow) {
		var maxTimeStamp int64 = time.Now().Unix()
		if !p.Renew && p.Maxid > 0 {
			st.GormDB.Model(&Note{}).
				Select(`MAX(notes.event_created_at) event_created_at`).
				Where("notes.id <= ?", p.Maxid).Scan(&maxTimeStamp)
		}
		since := maxTimeStamp - int64(p.Since*60*60*24)
		tx.Where("notes.event_created_at > ?", fmt.Sprintf("%d", since))
	}
	if p.Since == 0 && (!options.BookMark && !options.Follow) {
		var maxTimeStamp int64 = time.Now().Unix()
		since := maxTimeStamp - int64(1*60*60*24)
		tx.Where("notes.event_created_at > ?", fmt.Sprintf("%d", since))
	}

	if options.Follow {
		tx.Joins("JOIN follows ON (follows.pubkey = notes.pubkey)")
	} else {
		tx.Joins("LEFT JOIN follows ON (follows.pubkey = notes.pubkey)")
		if !options.BookMark {
			tx.Where("follows.pubkey is null")
		}
	}

	if options.BookMark {
		tx.Joins("JOIN bookmarks ON (bookmarks.note_id = notes.id)")
	} else {
		tx.Joins("LEFT JOIN bookmarks ON (bookmarks.note_id = notes.id)")
	}

	if !p.Renew {
		tx.Where("notes.id <= ?", fmt.Sprintf("%d", p.Maxid))
	}

	var count int64
	tx.Count(&count)

	tx.Limit(int(p.Limit))
	if p.Offset > 0 {
		tx.Offset(int(p.Offset))
	}

	tx.Order("notes.event_created_at DESC")
	rows, err := tx.Rows()
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	p.SetTotal(uint64(count))
	p.SetTo()
	eventMap, keys, seenMap, _ := st.procesEventRows(rows, p)
	tx = st.GormDB.WithContext(ctx).Model(&Seen{})
	seens := []*Seen{}
	for noteid, eventid := range seenMap {
		seens = append(seens, &Seen{NoteID: uint(noteid), EventId: eventid})
	}

	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seens)
	if result.Error != nil {
		return result.Error
	}
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
 * Do not show all data in an endless scrol page, but paginate it for easy access
 * and ignore the garbage tagged posts
 *
 */
func (st *Storage) GetPaginationRefeshPage(ctx context.Context, p *Pagination, ids *[]string, options Options) error {
	eventIds := ""
	for _, id := range *ids {
		eventIds = eventIds + `'` + id + `',`
	}
	eventIds = eventIds[:len(eventIds)-1]

	tx := st.GormDB.Debug().Table("notes").
		Select(`notes.id, notes.event_id, notes.pubkey, notes.kind, notes.event_created_at, 
		notes.content, notes.tags_full::json, notes.sig, notes.etags, notes.ptags,
		profiles.name, profiles.about , profiles.picture,
		profiles.website, profiles.nip05, profiles.lud16, profiles.display_name, 
		CASE WHEN length(follows.pubkey) > 0 THEN TRUE ELSE FALSE END followed, 
		CASE WHEN length(bookmarks.event_id) > 0 THEN TRUE ELSE FALSE END bookmarked`).
		Joins("LEFT JOIN profiles ON (profiles.pubkey = notes.pubkey)").
		Joins("LEFT JOIN blocks  on (blocks.pubkey = notes.pubkey)").
		//Joins("LEFT JOIN seens on (seens.event_id = notes.event_id)").
		Where("notes.kind = 1").
		//Where("seens.event_id IS NULL").
		Where("notes.garbage = false").
		Where("notes.event_id IN (" + eventIds + ")")

	if options.Follow {
		tx.Joins("JOIN follows ON (follows.pubkey = notes.pubkey)")
	} else {
		tx.Joins("LEFT JOIN follows ON (follows.pubkey = notes.pubkey)")
		tx.Where("follows.pubkey is null")
	}

	if options.BookMark {
		tx.Joins("JOIN bookmarks ON (bookmarks.event_id = notes.event_id)")
	} else {
		tx.Joins("LEFT JOIN bookmarks ON (bookmarks.event_id = notes.event_id)").
			Where("notes.root = true").
			Where("blocks.pubkey IS NULL")
	}

	var count int64
	tx.Count(&count)

	tx.Order("notes.event_created_at DESC")
	rows, err := tx.Rows()
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	p.SetTotal(uint64(count))
	p.SetTo()
	eventMap, keys, _, _ := st.procesEventRows(rows, p)
	st.getChildren(ctx, eventMap)

	events := make([]Event, 0)
	// Make sure the order stays the same
	for _, k := range keys {
		events = append(events, eventMap[k])
	}

	p.Data = events
	return nil
}

func (st *Storage) procesEventRows(rows *sql.Rows, p *Pagination) (map[string]Event, []string, map[uint64]string, error) {
	eventMap := make(map[string]Event)
	var keys []string
	seenMap := make(map[uint64]string)

	canUpdateMaxId := (p.Renew || p.Maxid == 0)
	for rows.Next() {
		var note Event
		var id uint64
		var name sql.NullString
		var about sql.NullString
		var picture sql.NullString

		var website sql.NullString
		var nip05 sql.NullString
		var lud16 sql.NullString
		var displayname sql.NullString
		var followed bool
		var bookmarked bool

		note.Event = &nostr.Event{}
		if err := rows.Scan(&id,
			&note.Event.ID, &note.Event.PubKey,
			&note.Event.Kind, &note.Event.CreatedAt,
			&note.Event.Content, &note.Event.Tags, &note.Event.Sig,
			(pq.Array)(&note.Etags), (pq.Array)(&note.Ptags),
			&name, &about, &picture, &website, &nip05, &lud16, &displayname, &followed, &bookmarked); err != nil {
			log.Println("procesEventRows() -> Query:: ", err.Error())
			panic(err)
		}

		if canUpdateMaxId && p.Maxid < id {
			p.Maxid = id
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

		note.Profile.Followed = followed
		note.Bookmark = bookmarked
		//nostr.Event = json.Unmarshal()
		note.Content = st.parseReferences(&note)

		seenMap[id] = note.Event.ID
		note.Children = make(map[string]*Event, 0)
		eventMap[note.Event.ID] = note
		keys = append(keys, note.Event.ID) // Make sure the order stays the same @see https://go.dev/blog/maps
	}
	// Check for errors from iterating over rows.
	if err := rows.Err(); err != nil {
		log.Println("procesEventRows() -> Query:: ", err)
		return nil, nil, nil, err
	}
	defer rows.Close()

	return eventMap, keys, seenMap, nil
}

func (st *Storage) parseReferences(note *Event) string {
	refs := sdk.ParseReferences(note.Event)
	note.Refs = Refs{}
	note.Refs.Profile = make(map[string]*Profile, 0)
	note.Refs.Event = make(map[string]*nostr.Event, 0)

	var content string = note.Event.Content
	for _, ref := range refs {
		if ref.Profile != nil {
			pubkey := ref.Profile.PublicKey
			if len(pubkey) == 64 {
				if profile, err := st.FindProfile(context.TODO(), pubkey); err == nil && profile.Name != "" {
					//content = event.Content[:ref.Start] + "[~" + profile.Name + "~]" + event.Content[ref.End:]
					content = strings.Replace(content, ref.Text, "[~["+profile.Pubkey+"]~]", -1)
					note.Refs.Profile[profile.Pubkey] = &Profile{
						Name:        profile.Name,
						About:       profile.About,
						Picture:     profile.Picture,
						Website:     profile.Website,
						Nip05:       profile.Nip05,
						Lud16:       profile.Lud16,
						DisplayName: profile.DisplayName,
						Pubkey:      profile.Pubkey,
						Blocked:     false,
						Followed:    false,
					}

				}
			}
		}
		if ref.Event != nil {
			if len(ref.Event.ID) == 64 {
				refEv, err := st.FindRawEvent(context.Background(), ref.Event.ID)
				if err == nil && refEv != nil {
					content = strings.Replace(content, ref.Text, "[~~["+refEv.Event.ID+"]~~]", -1)
					note.Refs.Event[refEv.Event.ID] = refEv.Event

					//content = event.Content[:ref.Start] + "[~" + refEv.Content[0:end] + "~]" + event.Content[ref.End:]
				}
				if refEv == nil || err == sql.ErrNoRows {
					content = strings.Replace(content, ref.Text, "[~~["+ref.Text[0:40]+"....]~~]", -1)
					//content = event.Content[:ref.Start] + "[~" + ref.Text[0:40] + "....~]" + event.Content[ref.End:]
				}
			}
		}
	}
	return content
}

func (st *Storage) getChildren(ctx context.Context, eventMap map[string]Event) error {
	var err error

	/**
	 * Get all child notes
	 */

	tx := st.GormDB.WithContext(ctx).Table("notes").
		Select(`trees.root_event_id, trees.reply_event_id, notes.id, notes.event_id, notes.pubkey, notes.kind, notes.event_created_at, 
		notes.content, notes.tags_full::json, notes.sig, notes.etags, notes.ptags,
		profiles.name, profiles.about , profiles.picture,
		profiles.website, profiles.nip05, profiles.lud16, profiles.display_name, 
		CASE WHEN length(follows.pubkey) > 0 THEN TRUE ELSE FALSE END followed, 
		CASE WHEN length(bookmarks.event_id) > 0 THEN TRUE ELSE FALSE END bookmarked`).
		Joins("JOIN trees ON (trees.event_id = notes.event_id)").
		Joins("LEFT JOIN profiles ON (profiles.pubkey = notes.pubkey)").
		Joins("LEFT JOIN blocks ON (blocks.pubkey = notes.pubkey)").
		//Joins("LEFT JOIN seens on (seens.event_id = notes.event_id)").
		Joins("LEFT JOIN follows ON (follows.pubkey = notes.pubkey)").
		Joins("LEFT JOIN bookmarks ON (bookmarks.event_id = notes.event_id)").
		Where("notes.kind = 1").
		Where("blocks.pubkey IS NULL").
		//Where("seens.event_id IS NULL").
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
		var followed bool
		var bookmarked bool
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

		childEvent.Profile.Followed = followed

		childEvent.Bookmark = bookmarked

		childEvent.Content = st.parseReferences(&childEvent)

		if item, ok := eventMap[root_event_id]; ok {

			if reply_event_id.Valid {
				if reply_event_id.String == "" {
					childEvent.Tree = 1
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
		log.Println("GetInbox() -> Query:: ", err)
		return nil
	}
	defer rows.Close()

	eventMap, keys, _, _ := st.procesEventRows(rows, p)
	p.SetTotal(uint64(len(keys)))
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
	switch err {
	case sql.ErrNoRows:
		log.Printf("FindEvent() -> Query:: 404 no event with id %s\n", id)
	default:
		log.Fatalf("FindEvent() -> Query:: 502 query error: %v\n", err)
	}
	event.Tree = 1
	event.RootId = event.Event.ID
	event.Children = make(map[string]*Event, 0)

	treeQry := `SELECT t.root_event_id, t.reply_event_id, 
	e.id, e.event_id, e.pubkey, e.kind, e.event_created_at, e.content,e.tags_full::json,e.sig, 
	e.etags, e.ptags , u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name, 
	CASE WHEN length(f.pubkey) > 0 THEN TRUE ELSE FALSE end followed
	CASE WHEN length(b.event_id) > 0 THEN TRUE ELSE FALSE end followed
	FROM trees t, notes e 
	LEFT JOIN profiles u ON (u.pubkey = e.pubkey ) 
	LEFT JOIN blocks b on (b.pubkey = e.pubkey) 
	LEFT JOIN seens s on (s.event_id = e.event_id)
	LEFT JOIN follows f ON (f.pubkey = e.pubkey)
	LEFT JOIN bookmarks b ON (b.note_id = e.id)
	WHERE root_event_id IN (` + `'` + event.Event.ID + `')` +
		` AND e.event_id = t.event_id
	AND e.kind = 1 AND b.pubkey IS NULL AND s.event_id IS NULL AND e.garbage = false;`

	var treeRows *sql.Rows
	treeRows, err = st.GormDB.WithContext(ctx).Raw(treeQry).Rows()
	if err != nil {
		log.Println("FindEvent() -> Query:: ", err)
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
		var followed bool
		var bookmarked bool

		childEvent.Event = &nostr.Event{}

		if err := treeRows.Scan(&root_event_id, &reply_event_id, &id,
			&childEvent.Event.ID, &childEvent.Event.PubKey, &childEvent.Event.Kind, &childEvent.Event.CreatedAt, &childEvent.Event.Content, &childEvent.Event.Tags, &childEvent.Event.Sig,
			(pq.Array)(&childEvent.Etags), (pq.Array)(&childEvent.Ptags), &name, &about, &picture,
			&website, &nip05, &lud16, &displayname, &followed, &bookmarked); err != nil {
			log.Println("FindEvent() -> Query:: ", err.Error())
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

		childEvent.Profile.Followed = followed
		childEvent.Bookmark = bookmarked
		event.Children[childEvent.Event.ID] = &childEvent
	}

	return event, err
}

func (st *Storage) FindRawEvent(ctx context.Context, id string) (*Event, error) {
	var qry = `SELECT e.event_id, e.pubkey, e.kind, e.event_created_at, e.content, e.sig, e.tags_full::json
	FROM notes e 
	WHERE e.event_id = $1`

	event := Event{}
	event.Event = &nostr.Event{}
	//ev := event.Event{}
	row := st.GormDB.WithContext(ctx).Raw(qry, id).Row()

	err := row.Scan(&event.Event.ID, &event.Event.PubKey, &event.Event.Kind, &event.Event.CreatedAt,
		&event.Event.Content, &event.Event.Sig, &event.Event.Tags)

	return &event, err
}

func (st *Storage) CheckProfiles(ctx context.Context, pubkeys []string, epochtime int64) ([]string, error) {
	qry := `SELECT pubkey FROM profiles WHERE EXTRACT(EPOCH FROM created_at) > $1 AND pubkey in (`

	for _, pubkey := range pubkeys {
		qry = qry + `'` + pubkey + `',`
	}
	qry = qry[:len(qry)-1] + `)`

	rows, err := st.GormDB.WithContext(ctx).Raw(qry, epochtime).Rows()
	if err != nil {
		log.Println("CheckProfile() -> Query:: ", err)
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
	if tx := st.GormDB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&followPubkey); tx.Error != nil {
		log.Println("CreateFollow() -> Create follow: ", tx.Error.Error())
	}

	return nil
}

func (st *Storage) RemoveFollow(ctx context.Context, pubkey string) error {
	if tx := st.GormDB.WithContext(ctx).Where("pubkey = ?", pubkey).Delete(&Follow{}); tx.Error != nil {
		log.Println("RemoveFollow() -> Remove follow: ", tx.Error.Error())
	}

	return nil
}

func (st *Storage) GetFollowedProfiles(ctx context.Context) []Profile {
	var profiles []Profile
	st.GormDB.
		WithContext(ctx).
		Model(&Profile{}).
		Joins("JOIN follows ON (follows.pubkey = profiles.pubkey)").
		Scan(&profiles)

	return profiles
}

func (st *Storage) CreateBookMark(ctx context.Context, eventID string) error {
	var note Note
	st.GormDB.WithContext(ctx).Where("event_id = ?", eventID).Find(&note)

	bookmark := Bookmark{EventId: eventID, NoteID: &note.ID}
	err := st.GormDB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&bookmark).Error
	if err != nil {
		log.Println("CreateBookMark() -> Query:: ", err)
		return err
	}

	return nil
}

func (st *Storage) RemoveBookMark(ctx context.Context, eventID string) error {
	err := st.GormDB.WithContext(ctx).Where("event_id = ?", eventID).Delete(&Bookmark{}).Error
	if err != nil {
		log.Println("RemoveBookMark() -> Query:: ", err)
		return err
	}

	return nil
}

func (st *Storage) CreateRelay(ctx context.Context, relay *Relay) error {
	tx := st.GormDB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&relay)

	log.Println(relay)
	err := tx.Error
	if err != nil {
		log.Println("CreateRelay() -> Query:: ", err)
		return err
	}
	return nil
}

func (st *Storage) RemoveRelay(ctx context.Context, url string) error {
	err := st.GormDB.WithContext(ctx).Where("url = ?", url).Delete(&Relay{}).Error
	if err != nil {
		log.Println("RemoveRelay() -> Query:: ", err)
		return err
	}

	return nil
}

func (st *Storage) GetRelays(ctx context.Context) []Relay {
	var relays []Relay
	st.GormDB.WithContext(ctx).Model(&Relay{}).Scan(&relays)

	return relays
}

func (st *Storage) GetLastTimeStamp(ctx context.Context) int64 {
	var createdAt int64
	st.GormDB.WithContext(ctx).Raw("SELECT MAX(event_created_at) as MaxCreated FROM notes").Scan(&createdAt)

	return createdAt
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
		log.Printf("FindProfile() -> Query:: 404 no profile with pubkey %s\n", pubkey)
	case err != nil:
		log.Fatalf("FindProfile() -> Query:: 502 query error: %v\n", err)
	}

	profile.Pubkey = pubkey
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
