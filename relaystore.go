package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"sync"
	"time"

	"github.com/lib/pq"
	"github.com/nbd-wtf/go-nostr"
)

type Profile struct {
	Name        string `json:"name"`
	About       string `json:"about"`
	Picture     string `json:"picture"`
	Website     string `json:"website"`
	Nip05       string `json:"nip05"`
	Lud16       string `json:"lud16"`
	DisplayName string `json:"display_name"`
}

type Event struct {
	ID        string   `json:"id"`
	Pubkey    string   `json:"pubkey"`
	Kind      int      `json:"kind"`
	CreatedAt int64    `json:"created_at"`
	Content   string   `json:"content"`
	Tags_full string   `json:"tags"`
	Etags     []string `json:"etags"`
	Ptags     []string `json:"ptags"`
	Sig       string   `json:"sig"`
	Profile   Profile  `json:"profile"`
}

type User struct {
	Name    string `json:"name"`
	About   string `json:"about"`
	Picture string `json:"picture"`
}

type BlockUser struct {
	Pubkey string `json:"pubkey"`
}

type DbConfig struct {
	User     string
	Password string
	Dbname   string
	Port     int
	Host     string
}

type Relay struct {
	Relay *nostr.Relay
	Url   string
	Read  bool
	Write bool
}

type Config struct {
	Database *DbConfig
	Relays   []string
	Pubkey   string
	Npub     string
	Pk       string
	Nsec     string
	Db       *sql.DB
}

const CreateQuery string = `
CREATE TABLE IF NOT EXISTS events (
	id TEXT PRIMARY KEY, 
	pubkey TEXT, 
	kind INTEGER, 
	created_at INTEGER, 
	content TEXT, 
	tags_full TEXT, 
	ptags text[],
	etags text[],
	sig TEXT,
	raw TEXT
);
CREATE INDEX IF NOT EXISTS events_by_kind ON events (kind, created_at);
CREATE INDEX IF NOT EXISTS events_by_ptags ON events (ptags, created_at);
CREATE INDEX IF NOT EXISTS events_by_etags ON events (etags, created_at);
CREATE INDEX IF NOT EXISTS events_by_pubkey_kind ON events (pubkey, kind, created_at);

CREATE TABLE IF NOT EXISTS users (
	id Integer Primary Key Generated Always as Identity, 
	pubkey VARCHAR UNIQUE, 
	name TEXT,
	about TEXT,
	picture TEXT,
	website TEXT,
	nip05 TEXT,
	lud16 TEXT,
	display_name TEXT,
	raw TEXT,
	created_at INTEGER 	
);
CREATE INDEX IF NOT EXISTS users_by_pubkey ON users (pubkey);

CREATE TABLE IF NOT EXISTS blockusers (
	id Integer Primary Key Generated Always as Identity, 
	pubkey VARCHAR UNIQUE, 
	created_at INTEGER
);
CREATE INDEX IF NOT EXISTS blockusers_by_pubkey ON blockusers (pubkey);
`

var EventsQueue = make([]nostr.Event, 0)
var myDb *sql.DB
var ptagsQueue = make([]string, 0)

func receiveEvents(sub *nostr.Subscription) {

	//var qry = `INSERT OR IGNORE INTO events (id, pubkey, kind, created_at, content,tags_full, sig, raw, p_tags, e_tags) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	//var ptags []string
	//var etags []string
	//ptags, etags := make([]string, 0), make([]string, 0)

	for ev := range sub.Events {
		log.Println("Receiving from channel")
		log.Println(ev.String())

		//ptags = ptags[:0]
		//etags = etags[:0]

		EventsQueue = append(EventsQueue, *ev)

		/*
			for _, tag := range ev.Tags {
				switch {
				case tag[0] == "e":
					if b, e := hex.DecodeString(tag[1]); e != nil || len(b) != 32 {
						continue
					} else {
						etags = append(etags, fmt.Sprintf("%x", b))
					}
				case tag[0] == "p":
					if b, e := hex.DecodeString(tag[1]); e != nil || len(b) != 32 {
						continue
					} else {
						ptags = append(ptags, fmt.Sprintf("%x", b))
					}
				}
			}
			etagsString := strings.Join(etags, "\n")
			ptagsString := strings.Join(ptags, "\n")

			tagJson, err := json.Marshal(ev.Tags)
			if err != nil {
				log.Fatal(err)
			}
			_, execErr := db.Exec(qry, ev.ID, ev.PubKey, ev.Kind, ev.CreatedAt, ev.Content, string(tagJson), ev.Sig, ev.String(), ptagsString, etagsString)
			if execErr != nil {
				log.Fatal(execErr)
			}
		*/
	}
}

/*
 * Please see https://github.com/mattn/algia/blob/main/main.go for the code i shamelessly copied
 */
func (cfg *Config) Do(f func(*nostr.Relay)) {
	var wg sync.WaitGroup

	for _, v := range cfg.Relays {
		wg.Add(1)
		go func(wg *sync.WaitGroup, v string) {
			defer wg.Done()
			ctx := context.WithValue(context.Background(), "url", v)
			relay, err := nostr.RelayConnect(ctx, v)
			if err != nil {
				log.Fatal(err)
				return
			}
			f(relay)
			relay.Close()
		}(&wg, v)
	}
	wg.Wait()
}

/* This function can go to storage.go */
func (cfg *Config) saveEvents(evs []*nostr.Event) []string {
	var qry = `INSERT INTO "events" ("id", "pubkey", "kind", "created_at", "content" , "tags_full" , "sig" , "raw" , "ptags" , "etags") 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT (id) DO NOTHING;`

	var pubkeys = make([]string, 0)

	tx, err := cfg.Db.Begin()
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
	for _, ev := range evs {
		log.Println("Event ID: ", ev.ID)
		pubkeys = append(pubkeys, fmt.Sprintf("%x", ev.PubKey))

		ptags = ptags[:0]
		etags = etags[:0]

		for _, tag := range ev.Tags {
			switch {
			case tag[0] == "e":
				if b, e := hex.DecodeString(tag[1]); e != nil || len(b) != 32 {
					continue
				} else {
					etags = append(etags, fmt.Sprintf("%x", b))
				}
			case tag[0] == "p":
				if b, e := hex.DecodeString(tag[1]); e != nil || len(b) != 32 {
					continue
				} else {
					ptags = append(ptags, fmt.Sprintf("%x", b))
					pubkeys = append(pubkeys, fmt.Sprintf("%x", b))
				}
			}
		}

		tagJson, err := json.Marshal(ev.Tags)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Add to transaction")
		if _, err := stmt.Exec(ev.ID, ev.PubKey, ev.Kind, ev.CreatedAt, ev.Content, string(tagJson), ev.Sig, ev.String(), pq.Array(ptags), pq.Array(etags)); err != nil {
			panic(err)
		}
	}

	log.Println("Ready to save events")
	if err := tx.Commit(); err != nil {
		panic(err)
	}
	return pubkeys
}

func (cfg *Config) getEvents(filter nostr.Filter) {
	log.Println("Get Event data from relays")
	var m sync.Map
	cfg.Do(func(relay *nostr.Relay) {
		evs, err := relay.QuerySync(context.Background(), filter)
		if err != nil {
			return
		}
		for _, ev := range evs {
			if _, ok := m.Load(ev.ID); !ok {
				/* if ev.Kind == nostr.KindEncryptedDirectMessage {
					if err := cfg.Decode(ev); err != nil {
						continue
					}
				} */
				m.LoadOrStore(ev.ID, ev)
			}
		}
	})

	var evs []*nostr.Event
	m.Range(func(k, v any) bool {
		log.Println(k)
		evs = append(evs, v.(*nostr.Event))
		return true
	})

	var pubkeys = make([]string, 0)
	pubkeys = cfg.saveEvents(evs)

	cfg.updateUsers(pubkeys)

	defer func() {
		log.Println("Done receiving and closed ralay connections")
	}()
}

func getFilters(createdAt int64) nostr.Filters {
	//var timeStamp nostr.Timestamp = nostr.Timestamp(time.Now().Unix() - 60)
	var timeStamp nostr.Timestamp = nostr.Timestamp(createdAt + 1)

	var filters nostr.Filters
	filters = []nostr.Filter{{
		Kinds: []int{nostr.KindTextNote, nostr.KindReaction, nostr.KindArticle},
		Since: &timeStamp,
	}}

	return filters
}

func (cfg *Config) getEventData() {
	var createdAt int64
	row := cfg.Db.QueryRow("SELECT MAX(created_at) as MaxCreated FROM events")
	row.Scan(&createdAt)
	log.Println(createdAt)
	if createdAt < 1 {
		createdAt = time.Now().Unix() - 60
	}
	if createdAt > time.Now().Unix()-60 {
		log.Printf("Time lapse is to short for getting new data %d %d", createdAt, time.Now().Unix()-60)
		return
	}

	var timeStamp nostr.Timestamp = nostr.Timestamp(createdAt + 1)
	filter := nostr.Filter{
		Kinds: []int{nostr.KindTextNote, nostr.KindReaction, nostr.KindArticle},
		Since: &timeStamp,
	}

	cfg.getEvents(filter)

	defer func() {
		log.Println("Closing shop")
	}()
}

/* This function can go to storage.go */
func (cfg *Config) saveProfiles(evs []*nostr.Event) {
	var qry = `INSERT INTO "users" ("pubkey", "name","about", "picture",  "website", "nip05",
	"lud16", "display_name", "raw", "created_at")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT (pubkey) DO NOTHING;`

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var tx *sql.Tx
	tx, err := cfg.Db.Begin()
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

func (cfg *Config) updateUsers(pubkeys []string) {
	filter := nostr.Filter{
		Kinds:   []int{nostr.KindSetMetadata},
		Authors: pubkeys,
	}

	log.Println("Get user data from relays")
	var m sync.Map
	cfg.Do(func(relay *nostr.Relay) {
		evs, err := relay.QuerySync(context.Background(), filter)
		if err != nil {
			return
		}
		for _, ev := range evs {
			if _, ok := m.Load(ev.ID); !ok {
				m.LoadOrStore(ev.ID, ev)
			}
		}
	})

	var evs []*nostr.Event
	m.Range(func(k, v any) bool {
		log.Println(k)
		evs = append(evs, v.(*nostr.Event))
		return true
	})

	cfg.saveProfiles(evs)
	log.Println("Done for users")
}

func getLast(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	tx, err := myDb.Begin()
	if err != nil {
		panic(err)
	}

	rows, err := tx.Query(`SELECT e.id, e.pubkey, e.kind, e.created_at, e.content, e.tags_full, e.etags, e.ptags, e.sig, u.name, u.about , u.picture,
	u.website, u.nip05, u.lud16, u.display_name
	FROM events e LEFT JOIN users u ON (u.pubkey = e.pubkey ) LEFT JOIN blockusers b on (b.pubkey = e.pubkey) 
	WHERE e.kind = 1 AND b.pubkey IS NULL ORDER BY e.created_at DESC LIMIT 30`)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer rows.Close()

	//log.Println("We got rows")

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
			log.Fatal(err)
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
		log.Fatal(err)
	}

	//encoder := json.NewEncoder(w)
	//encoder.SetEscapeHTML(false)
	//encoder.Encode(events)

	json.NewEncoder(w).Encode(events)
}

func searchEvent(id string, db *sql.DB) Event {
	row := db.QueryRow(`SELECT e.id, e.pubkey, e.kind, e.created_at, e.content, e.tags_full, e.etags, e.ptags, e.sig, u.name, u.about , u.picture
	FROM events e LEFT JOIN users u ON (u.pubkey = e.pubkey ) LEFT JOIN blockusers b on (b.pubkey = e.pubkey) 
	WHERE e.id = $1`, id)
	var ev Event
	row.Scan(&ev)

	if ev.ID != "" {
		log.Println("200 Found the event you are searching for ;)")
	}
	if ev.ID == "" {
		log.Println("404 Event not found")
	}
	return ev
}

func CheckError(err error) {
	if err != nil {
		panic(err)
	}
}

func blockUser(db *sql.DB, user *BlockUser) {
	_, err := myDb.Exec(`INSERT INTO "blockusers" (pubkey, created_at) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING;`, user.Pubkey, time.Now().Unix())
	if err != nil {
		log.Fatal(err)
	}
}

func loadConfig() (*Config, error) {
	var cfg Config

	content, err := ioutil.ReadFile("./config.json")
	if err != nil {
		fmt.Println("Done", err)
		log.Fatal("Error when opening file: ", err)
		return nil, err
	}

	err = json.Unmarshal(content, &cfg)
	if err != nil {
		log.Fatal("Error during Unmarshal(): ", err)
		return nil, err
	}

	//log.Println("Content nieuw", *settings)
	// Let's print the unmarshalled data!
	log.Printf("dbName: %s\n", cfg.Database.Dbname)
	log.Printf("Pubkey: %s\n", cfg.Pubkey)
	return &cfg, nil
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		panic(err)
	}

	// connection string
	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.Dbname)

	// open database
	db, err := sql.Open("postgres", psqlconn)
	CheckError(err)

	cfg.Db = db

	// close database
	defer db.Close()

	// check db
	err = db.Ping()
	CheckError(err)

	fmt.Println("Connected!")

	_, err = db.Exec(CreateQuery)
	if err != nil {
		log.Fatal(err)
	}

	myDb = db

	// Windows may be missing this
	mime.AddExtensionType(".js", "application/javascript")
	http.HandleFunc("/api/follow", getLast)
	http.HandleFunc("/api/getnext", func(w http.ResponseWriter, r *http.Request) {

		EventsQueue = EventsQueue[:0]
		cfg.getEventData()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)
		test := make(map[string]string)
		test["status"] = "ok"
		test["message"] = "This will take a while"
		json.NewEncoder(w).Encode(test)
	})

	http.HandleFunc("/api/blockuser", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var j BlockUser
		err = json.NewDecoder(r.Body).Decode(&j)
		if err != nil {
			panic(err)
		}

		blockUser(db, &j)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)
		test := map[string]string{}
		test["status"] = "ok"
		test["blocked"] = j.Pubkey
		json.NewEncoder(w).Encode(test)
	})

	http.HandleFunc("/api/searchevent", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		type Request struct {
			ID string
		}
		var j Request
		err = json.NewDecoder(r.Body).Decode(&j)
		if err != nil {
			panic(err)
		}
		log.Println("Searching event with Id: ", j.ID)
		ev := searchEvent(j.ID, db)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ev)
	})

	http.Handle("/", http.FileServer(http.Dir("web/nostr-reader/dist")))

	log.Fatal(http.ListenAndServe(":8080", nil))

}
