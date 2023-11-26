package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"strings"
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
	Pubkey      string `json:"pubkey"`
}

type Event struct {
	EventID        string   `json:"id"`
	Pubkey         string   `json:"pubkey"`
	Kind           int      `json:"kind"`
	EventCreatedAt int64    `json:"created_at"`
	Content        string   `json:"content"`
	TagsFull       string   `json:"tags"`
	Etags          []string `json:"etags"`
	Ptags          []string `json:"ptags"`
	Sig            string   `json:"sig"`
	Profile        Profile  `json:"profile"`
	Garbage        bool     `json:"gargabe"`
}

type BlockPubkey struct {
	Pubkey string `json:"pubkey"`
}

type FollowPubkey struct {
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
	Filter   []string
	Storage  *Storage
}

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

var EventsQueue = make([]nostr.Event, 0)
var ptagsQueue = make([]string, 0)
var syncHash string = ""

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
				log.Println(err)
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

	cfg.updateProfiles(pubkeys)

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

func (cfg *Config) updateProfiles(pubkeys []string) {
	filter := nostr.Filter{
		Kinds:   []int{nostr.KindProfileMetadata},
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
	log.Println("Done for profiles")
}

func (cfg *Config) getLast(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	events, err := cfg.Storage.GetEvents(30)
	if err != nil {
		log.Println(err)
		return
	}

	json.NewEncoder(w).Encode(events)
}

func (cfg *Config) blockPubkey(user *BlockPubkey) {
	_, err := cfg.Storage.Db.Exec(`INSERT INTO "block_pubkeys" (pubkey, created_at) VALUES ($1, NOW()) ON CONFLICT (id) DO NOTHING;`, user.Pubkey)
	if err != nil {
		log.Println(err)
	}
}

func loadConfig() (*Config, error) {
	var cfg Config

	content, err := ioutil.ReadFile("./config.json")
	if err != nil {
		fmt.Println("Done", err)
		log.Println("Error when opening file: ", err)
		return nil, err
	}

	err = json.Unmarshal(content, &cfg)
	if err != nil {
		log.Println("Error during Unmarshal(): ", err)
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

	st.Filter = cfg.Filter
	cfg.Storage = &st

	// close database
	defer st.Db.Close()

	_, err = st.Db.Exec(CreateQuery)
	if err != nil {
		panic(err)
	}

	// Windows may be missing this
	mime.AddExtensionType(".js", "application/javascript")
	//http.HandleFunc("/api/events", cfg.getLast)
	http.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		type Page struct {
			Page  int
			Limit int
			Since int
		}
		var p Page
		err = json.NewDecoder(r.Body).Decode(&p)
		if err != nil {
			panic(err)
		}

		pagination := Pagination{}
		pagination.SetLimit(p.Limit)
		pagination.SetCurrentPage(p.Page)
		pagination.SetSince(p.Since)
		err := cfg.Storage.GetEventPagination(&pagination)

		//pagination.PerPage = 20
		//pagination.CurrentPage = page
		//pagination.LastPage = int64(math.Floor(float64(pagination.Total) / 20.0))
		//pagination.From = (page - 1) * 20
		//pagination.To = (page-1)*20 + 20 // Not correct at end

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)

		if err != nil {
			log.Println(err)
		}
		json.NewEncoder(w).Encode(&pagination)
	})

	http.HandleFunc("/api/sync", func(w http.ResponseWriter, r *http.Request) {

		EventsQueue = EventsQueue[:0]
		cfg.getEventData()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)

		syncHash = fmt.Sprint(time.Now().Unix())

		test := make(map[string]string)
		test["status"] = "ok"
		test["message"] = syncHash
		json.NewEncoder(w).Encode(test)
	})

	http.HandleFunc("/api/blockuser", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var j BlockPubkey
		err = json.NewDecoder(r.Body).Decode(&j)
		if err != nil {
			panic(err)
		}

		cfg.blockPubkey(&j)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)
		test := map[string]string{}
		test["status"] = "ok"
		test["blocked"] = j.Pubkey
		json.NewEncoder(w).Encode(test)
	})

	http.HandleFunc("/api/followuser", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var user FollowPubkey
		err = json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			panic(err)
		}

		cfg.Storage.FollowPubkey(user.Pubkey)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)
		test := map[string]string{}
		test["status"] = "ok"
		test["followed"] = user.Pubkey
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
		ev := cfg.Storage.FindEvent(j.ID)
		if ev.EventID == "" {
			var tagMap nostr.TagMap
			if tagMap == nil {
				tagMap = make(nostr.TagMap)
			}
			tagMap["e"] = append(tagMap["e"], j.ID)
			filter := nostr.Filter{
				Tags:  tagMap,
				Limit: 1,
			}
			log.Println(filter)
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

			pubkeys := cfg.Storage.SaveEvents(evs)
			cfg.updateProfiles(pubkeys)

		}
		ev = cfg.Storage.FindEvent(j.ID)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ev)
	})

	http.HandleFunc("/api/preview/link", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		type Url struct {
			Url string
		}
		var url Url
		err = json.NewDecoder(r.Body).Decode(&url)
		if err != nil {
			panic(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")

		w.WriteHeader(http.StatusOK)
		t := strings.TrimSpace(url.Url)
		s := strings.Split(t, "\n")
		log.Println("Url to preview: ", s[0])

		result, err := URLPreview(s[0])
		if err != nil {
			log.Println(err)

		}
		log.Println("Preview result: ", result)
		json.NewEncoder(w).Encode(result)
	})

	http.Handle("/", http.FileServer(http.Dir("web/nostr-reader/dist")))

	log.Fatal(http.ListenAndServe(":8080", nil))

}
