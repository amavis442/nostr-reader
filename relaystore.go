package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"sync"
	"time"

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
	Storage  *Storage
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
	pubkeys = cfg.Storage.SaveEvents(evs)

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
	row := cfg.Storage.Db.QueryRow("SELECT MAX(created_at) as MaxCreated FROM events")
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

	cfg.Storage.SaveProfiles(evs)
	log.Println("Done for users")
}

func (cfg *Config) getLast(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	events, err := cfg.Storage.GetEvents(30)
	if err != nil {
		log.Fatal(err)
	}

	json.NewEncoder(w).Encode(events)
}

func (cfg *Config) searchEvent(id string) Event {
	row := cfg.Storage.Db.QueryRow(`SELECT e.id, e.pubkey, e.kind, e.created_at, e.content, e.tags_full, e.etags, e.ptags, e.sig, u.name, u.about , u.picture
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

func (cfg *Config) blockUser(user *BlockUser) {
	_, err := cfg.Storage.Db.Exec(`INSERT INTO "blockusers" (pubkey, created_at) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING;`, user.Pubkey, time.Now().Unix())
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

	var st Storage
	st.Connect(cfg)

	cfg.Storage = &st

	// close database
	defer st.Db.Close()

	_, err = st.Db.Exec(CreateQuery)
	if err != nil {
		log.Fatal(err)
	}

	myDb = st.Db

	// Windows may be missing this
	mime.AddExtensionType(".js", "application/javascript")
	http.HandleFunc("/api/follow", cfg.getLast)
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

		cfg.blockUser(&j)

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
		ev := cfg.searchEvent(j.ID)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ev)
	})

	http.Handle("/", http.FileServer(http.Dir("web/nostr-reader/dist")))

	log.Fatal(http.ListenAndServe(":8080", nil))

}
