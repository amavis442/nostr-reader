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
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
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

	log.Println("Connect() -> Connected to database:", cfg.Dbname)
	return err
}

func (st *Storage) SaveProfile(ctx context.Context, ev *Event) error {
	var data Profile
	content := ev.Event.Content
	err := json.Unmarshal([]byte(content), &data)
	if err != nil {
		log.Printf(Red + getCallerInfo(1) + " SaveProfile() --> Unmarshal:: " + err.Error() + Reset + "\n")
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
	jsonBufBytes := jsonbuf.Bytes()
	newUUID, _ := uuid.NewV7()
	profile := &Profile{
		Pubkey:      ev.Event.PubKey,
		UID:         newUUID,
		Name:        data.Name,
		About:       data.About,
		Picture:     data.Picture,
		Website:     data.Website,
		Nip05:       data.Nip05,
		Lud16:       data.Lud16,
		DisplayName: data.DisplayName,
		Raw:         jsonBufBytes,
		Urls:        ev.Urls,
	}
	profile.UpdatedAt.Time = time.Now()

	slog.Info(Yellow + "SaveProfile() -> Adding or updating profile: pubkey = " + profile.Pubkey + Reset)

	var searchProfile Profile
	if err := st.GormDB.Model(&Profile{}).Where(Profile{Pubkey: ev.Event.PubKey}).Find(&searchProfile).Error; err != nil {
		switch {
		case err == sql.ErrNoRows:
			log.Printf(Yellow+"FindProfile() -> Query:: 404 no profile with pubkey %s\n"+Reset, ev.Event.PubKey)
		default:
			_, file, line, _ := runtime.Caller(0)
			log.Fatalf(Yellow+"FindProfile(%s::%d) -> Query:: 502 query error: %v\n"+Reset, file, line, err)
		}
		return err
	}

	result := st.GormDB.Where(Profile{Pubkey: ev.Event.PubKey}).
		Assign(profile).
		FirstOrCreate(&profile)
	if result.Error != nil {
		return result.Error
	}

	if st.GormDB.Error != nil {
		log.Print(st.GormDB.Error.Error())
		return st.GormDB.Error
	}

	picture := data.Picture.String
	// Should be in a dynamic list, so you can add to it or remove items.
	if picture != "" && len(picture) > len("https://randomuser.me") && picture[0:len("https://randomuser.me")] == "https://randomuser.me" {
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
		slog.Warn(getCallerInfo(1), "error", err.Error())
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
		slog.Warn(getCallerInfo(1), "error", err.Error())
		return Note{}, err
	}

	newUUID, _ := uuid.NewV7()
	note := Note{}
	note.EventId = ev.ID
	note.UID = newUUID
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
			log.Fatalf(getCallerInfo(1)+" FindProfile() -> Query:: 502 query error: %v\n", err)
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
		slog.Warn(getCallerInfo(1), "error", err.Error())
		return Note{}, err
	}

	if note.ID > 0 && len(tree.RootTag) > 0 {
		treeData := Tree{EventId: ev.ID, RootEventId: tree.RootTag, ReplyEventId: tree.ReplyTag}
		err = st.GormDB.Model(&Tree{}).WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&treeData).Error
		if err != nil {
			slog.Error(getCallerInfo(1), "error", err.Error())
		}

		if hasNotification {
			notification := Notification{NoteID: note.ID}
			err = st.GormDB.Model(&Notification{}).WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&notification).Error
			if err != nil {
				slog.Error(getCallerInfo(1), "error", err.Error())
			}
		}

		// Check if we already have the root Note
		var searchNoteRootNote Note
		err = st.GormDB.Model(&Note{}).Where(&Note{EventId: tree.RootTag}).Find(&searchNoteRootNote).Error
		if err != nil {
			slog.Error(getCallerInfo(1), "error", err.Error())
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
				slog.Error(getCallerInfo(1), "error", err.Error())
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
	Renew    bool
}

func (st *Storage) GetNewNotesCount(ctx context.Context, cursor uint64, options Options) (int, error) {
	var count int
	tx := st.GormDB.Model(&NotesAndProfiles{}).
		Select(`COUNT(id)`).
		Where("id > ?", cursor).
		Where("followed = ? and bookmarked = ?", options.Follow, options.BookMark).
		Find(&count)

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

func (st *Storage) initPaging(p *Pagination, options Options) {
	if p.Cursor == 0 {
		var notesAndProfiles []NotesAndProfiles
		fmt.Println("Empty cursor")
		if !options.BookMark {
			//var maxTimeStamp int64 = time.Now().Unix()
			//currentTime := time.Now()
			//year := currentTime.Year()
			//month := currentTime.Month()
			//day := currentTime.Day()
			//loc, _ := time.LoadLocation("Local")
			//since := time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc).Unix()
			//since := maxTimeStamp - int64(60*60*24*p.GetSince())
			st.GormDB.Debug().Model(&NotesAndProfiles{}).
				Where(`id < (SELECT MAX(id) FROM "notes_and_profiles" WHERE followed = @follow and bookmarked = @bookmark)`, sql.Named("follow", options.Follow), sql.Named("bookmark", options.BookMark)).
				Where("followed = @follow and bookmarked = @bookmark", sql.Named("follow", options.Follow), sql.Named("bookmark", options.BookMark)).
				Order("id DESC").
				Limit(30).
				Find(&notesAndProfiles)

			p.Cursor = notesAndProfiles[len(notesAndProfiles)-1].ID
			/*
				if notesAndProfiles.ID == 0 {
					st.GormDB.Model(&NotesAndProfiles{}).
						Where("event_created_at > ?", since).
						Where("followed = ? and bookmarked = ?", options.Follow, options.BookMark).
						Order("id DESC").
						Limit(1).
						Find(&notesAndProfiles)
					p.Cursor = notesAndProfiles.ID
				}
			*/
		}
	}
}

type NotesAndProfiles struct {
	ID             uint64          `gorm:"type:bigint"`
	NoteUUID       uuid.UUID       `gorm:"type:uuid"`
	EventId        string          `gorm:"type:text"`
	Pubkey         string          `gorm:"type:varchar(100)"`
	Kind           int             `gorm:"type:int"`
	EventCreatedAt nostr.Timestamp `gorm:"type:bigint"`
	Content        string          `gorm:"type:text"`
	TagsFull       nostr.Tags      `gorm:"type:text"`
	Sig            string          `gorm:"type:varchar(200)"`
	Etags          pq.StringArray  `gorm:"type:text[]"`
	Ptags          pq.StringArray  `gorm:"type:text[]"`
	ProfileUUID    uuid.UUID       `gorm:"type:uuid"`
	Name           NullString      `gorm:"type:varchar(255)"`
	About          NullString      `gorm:"type:text"`
	Picture        NullString      `gorm:"type:varchar(255)"`
	Website        NullString      `gorm:"type:varchar(255)"`
	Nip05          NullString      `gorm:"type:varchar(255)"`
	Lud16          NullString      `gorm:"type:varchar(255)"`
	DisplayName    NullString      `gorm:"type:varchar(255)"`
	Followed       bool            `gorm:"type:bool;"`
	Bookmarked     bool            `gorm:"type:bool;"`
}

type ServerState int

const (
	StateInit ServerState = iota
	StateNext
	StatePrev
	StateRefresh
)

var stateName = map[ServerState]string{
	StateInit:    "init",
	StateNext:    "next",
	StatePrev:    "prev",
	StateRefresh: "refresh",
}

/**
 * Do not show all data in an endless scroll page, but paginate it for easy access
 * and ignore the garbage tagged posts
 *
 */
func (st *Storage) GetNotes(ctx context.Context, context string, p *Pagination, options Options) (*[]Event, error) {
	var state string
	if p.Cursor == 0 {
		state = stateName[StateInit]
	}
	if p.Cursor > 0 {
		state = stateName[StateRefresh]
	}
	if p.NextCursor != 0 {
		state = stateName[StateNext]
	}
	if p.PreviousCursor != 0 {
		state = stateName[StatePrev]
	}
	st.initPaging(p, options)
	slog.Info("State is: ", "state", state)

	tx := st.GormDB.Debug().Where("followed = ? and bookmarked = ?", options.Follow, options.BookMark)

	if state == stateName[StateInit] || state == stateName[StateRefresh] {
		tx.Where("id > ?", p.Cursor).
			Order("id ASC")
	}

	if state == stateName[StateNext] {
		tx.Where("id > ?", p.NextCursor).
			Order("id ASC")
	}
	if state == stateName[StatePrev] {
		tx.Where("id < ?", p.PreviousCursor).
			Order("id DESC")
	}

	tx.Limit(int(p.GetPerPage())) // Last one is not shown and only used for the next cursor

	var rows []NotesAndProfiles
	tx.Find(&rows)
	if len(rows) == 0 {
		return &[]Event{}, nil
	}

	p.NextCursor = 0
	p.PreviousCursor = 0

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ID > rows[j].ID
	})

	if (state == stateName[StateInit] || state == stateName[StateNext] || state == stateName[StateRefresh]) && !(len(rows) < int(p.PerPage)) {
		next_cursor := rows[0]
		p.NextCursor = next_cursor.ID
	}

	if (state == stateName[StatePrev] || state == stateName[StateRefresh]) && !(len(rows) < int(p.PerPage)) {
		next_cursor := rows[0]
		p.NextCursor = next_cursor.ID
	}

	if state == stateName[StateInit] || state == stateName[StateNext] || state == stateName[StateRefresh] {
		prev_cursor := rows[len(rows)-1]
		p.PreviousCursor = prev_cursor.ID
	}
	if (state == stateName[StatePrev] || state == stateName[StateRefresh]) && !(len(rows) < int(p.PerPage)) {
		prev_cursor := rows[len(rows)-1]
		p.PreviousCursor = prev_cursor.ID
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].EventCreatedAt.Time().Unix() > rows[j].EventCreatedAt.Time().Unix()
	})

	eventMap, keys, seenMap, err := st.procesEventRows(&rows)
	if err != nil {
		log.Fatal(err.Error())
	}

	tx = st.GormDB.WithContext(ctx).Model(&Seen{})
	seens := []*Seen{}
	for noteid, eventid := range seenMap {
		seens = append(seens, &Seen{NoteID: uint(noteid), EventId: eventid})
	}

	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seens)
	if result.Error != nil {
		return &[]Event{}, result.Error
	}

	st.getChildren(ctx, eventMap)

	events := make([]Event, 0)
	// Make sure the order stays the same
	for _, k := range keys {
		events = append(events, eventMap[k])
	}

	return &events, nil
}

func (st *Storage) GetNotifications(ctx context.Context, p *Pagination) (*[]Event, error) {
	var state string
	if p.Cursor == 0 {
		state = stateName[StateInit]
	}
	if p.Cursor > 0 {
		state = stateName[StateRefresh]
	}
	if p.NextCursor != 0 {
		state = stateName[StateNext]
	}
	if p.PreviousCursor != 0 {
		state = stateName[StatePrev]
	}
	slog.Info("State is: ", "state", state)

	tx := st.GormDB.Debug().Model(&Note{}).
		Joins("JOIN notifications ON (notifications.note_id = notes.id)").
		Limit(int(p.GetPerPage())) // Last one is not shown and only used for the next cursor

	var notes []Note
	tx.Find(&notes)
	if len(notes) == 0 {
		return &[]Event{}, nil
	}

	var root_tags []string
	for _, row := range notes {
		//var etags []string
		var ev *nostr.Event
		json.Unmarshal(row.Raw, &ev)
		_, _, _, _, tree := ProcessTags(ev, st.Pubkey)

		fmt.Println(tree.RootTag)
		root_tags = append(root_tags, tree.RootTag)
	}
	var rows []NotesAndProfiles
	err := st.GormDB.Debug().Model(&NotesAndProfiles{}).Where("event_id IN (?)", root_tags).Find(&rows).Error
	if err != nil {
		slog.Error(getCallerInfo(1), "error", err.Error())
		return nil, nil
	}

	if len(notes) == 0 {
		return &[]Event{}, nil
	}

	p.NextCursor = 0
	p.PreviousCursor = 0

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ID > rows[j].ID
	})

	if (state == stateName[StateInit] || state == stateName[StateNext] || state == stateName[StateRefresh]) && !(len(rows) < int(p.PerPage)) {
		next_cursor := rows[0]
		p.NextCursor = next_cursor.ID
	}

	if (state == stateName[StatePrev] || state == stateName[StateRefresh]) && !(len(rows) < int(p.PerPage)) {
		next_cursor := rows[0]
		p.NextCursor = next_cursor.ID
	}

	if state == stateName[StateInit] || state == stateName[StateNext] || state == stateName[StateRefresh] {
		prev_cursor := rows[len(rows)-1]
		p.PreviousCursor = prev_cursor.ID
	}
	if (state == stateName[StatePrev] || state == stateName[StateRefresh]) && !(len(rows) < int(p.PerPage)) {
		prev_cursor := rows[len(rows)-1]
		p.PreviousCursor = prev_cursor.ID
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].EventCreatedAt.Time().Unix() > rows[j].EventCreatedAt.Time().Unix()
	})

	eventMap, keys, _, err := st.procesEventRows(&rows)
	if err != nil {
		log.Fatal(err.Error())
	}

	st.getChildren(ctx, eventMap)

	events := make([]Event, 0)
	// Make sure the order stays the same
	for _, k := range keys {
		events = append(events, eventMap[k])
	}

	return &events, nil
}

func (st *Storage) procesEventRows(rows *[]NotesAndProfiles) (map[string]Event, []string, map[uint64]string, error) {
	eventMap := make(map[string]Event)
	var keys []string
	seenMap := make(map[uint64]string)

	for _, item := range *rows {
		var note Event
		note.Event = &nostr.Event{}
		note.Profile = Profile{}
		note.Event.ID = item.EventId
		if _, ok := eventMap[note.Event.ID]; ok {
			continue
		}
		note.Event.Content = item.Content
		note.Event.Kind = item.Kind
		note.Event.PubKey = item.Pubkey
		note.Profile.Pubkey = item.Pubkey
		note.Event.Tags = item.TagsFull
		note.Event.Sig = item.Sig
		note.Event.CreatedAt = item.EventCreatedAt

		note.RootId = note.Event.ID
		note.Tree = 1

		if item.ProfileUUID.String() != "" {
			note.Profile.UID = item.ProfileUUID
		}

		if item.Name.Valid {
			note.Profile.Name = item.Name
		} else {
			note.Profile.Name.String = item.Pubkey
		}
		if item.About.Valid {
			note.Profile.About = item.About
		}
		if item.Picture.Valid {
			note.Profile.Picture = item.Picture
		}
		if item.Website.Valid {
			note.Profile.Website = item.Website
		}
		if item.Nip05.Valid {
			note.Profile.Nip05 = item.Nip05
		}
		if item.Lud16.Valid {
			note.Profile.Lud16 = item.Lud16
		}
		if item.DisplayName.Valid {
			note.Profile.DisplayName = item.DisplayName
		}

		note.Profile.Followed = item.Followed
		note.Bookmark = item.Bookmarked
		//nostr.Event = json.Unmarshal()
		note.Content = st.parseReferences(&note)

		seenMap[item.ID] = note.Event.ID
		note.Children = make(map[string]*Event, 0)
		eventMap[note.Event.ID] = note
		keys = append(keys, note.Event.ID) // Make sure the order stays the same @see https://go.dev/blog/maps
	}
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
				if profile, err := st.FindProfile(context.TODO(), pubkey); err == nil && profile.Name.String != "" {
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
		var name NullString
		var about NullString
		var picture NullString

		var website NullString
		var nip05 NullString
		var lud16 NullString
		var displayname NullString
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
		childEvent.Profile = Profile{}
		childEvent.Profile.Pubkey = childEvent.Event.PubKey
		if name.Valid {
			childEvent.Profile.Name = name
		} else {
			childEvent.Profile.Name.String = childEvent.Event.PubKey
		}
		if about.Valid {
			childEvent.Profile.About = about
		}
		if picture.Valid {
			childEvent.Profile.Picture = picture
		}

		if website.Valid {
			childEvent.Profile.Website = website
		}
		if nip05.Valid {
			childEvent.Profile.Nip05 = nip05
		}
		if lud16.Valid {
			childEvent.Profile.Lud16 = lud16
		}
		if displayname.Valid {
			childEvent.Profile.DisplayName = displayname
		}

		childEvent.Profile.Followed = followed

		childEvent.Bookmark = bookmarked

		/*
			if childEvent.Event.PubKey == "39fa74ddf269649ce45d40b9de4ae8a8f94e7713d74d18901f1f24906c97e3e4" {
				log.Fatal("Pubkey: ", childEvent.Event.PubKey, childEvent.Profile.Name)
			}
		*/
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

func (st *Storage) GetInbox(ctx context.Context, context string, p *Pagination, pubkey string) (*[]Event, error) {
	qry := `
	SELECT
        np.*
        FROM
        notes_and_profiles np
        JOIN
        (
			SELECT
        	DISTINCT e1.event_id
        	FROM
        	notes e1
        	JOIN (
		        SELECT t.root_event_id, t.event_id, t.reply_event_id FROM trees t, notes e2
       		 	WHERE e2.pubkey = $1
        		AND e2.event_id = t.event_id
			) t0 ON e1.event_id = t0.root_event_id
        ) tbl
        ON
        (tbl.event_id = np.event_id)
        ORDER BY np.event_created_at DESC`

	var rows []NotesAndProfiles
	//rows, err := tx.Query(ctx, qry, pubkey)
	err := st.GormDB.Debug().WithContext(ctx).Raw(qry, pubkey).Limit(100).Find(&rows).Error
	if err != nil {
		slog.Error(getCallerInfo(1), "error", err)
	}

	eventMap, keys, _, _ := st.procesEventRows(&rows)

	st.getChildren(ctx, eventMap)
	events := make([]Event, 0)
	// Make sure the order stays the same
	for _, k := range keys {
		events = append(events, eventMap[k])
	}

	return &events, nil
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
			childEvent.Profile.Name.String = name.String
		} else {
			childEvent.Profile.Name.String = event.Event.PubKey
		}
		if about.Valid {
			childEvent.Profile.About.String = about.String
		}
		if picture.Valid {
			childEvent.Profile.Picture.String = picture.String
		}

		if website.Valid {
			childEvent.Profile.Website.String = website.String
		}
		if nip05.Valid {
			childEvent.Profile.Nip05.String = nip05.String
		}
		if lud16.Valid {
			childEvent.Profile.Lud16.String = lud16.String
		}
		if displayname.Valid {
			childEvent.Profile.DisplayName.String = displayname.String
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
	var profile Profile
	err := st.GormDB.Debug().Model(&Profile{}).Where("pubkey = ?", pubkey).Find(&profile).Error

	switch {
	case err == sql.ErrNoRows:
		slog.Info(fmt.Sprintf("FindProfile() -> Query:: 404 no profile with pubkey %s\n", pubkey))
	default:
		slog.Info(fmt.Sprintf("FindProfile() -> Query:: 502 query error: %v\n", err))
	}

	return profile, err
}
