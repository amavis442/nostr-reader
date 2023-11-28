package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

/**
 * Main app
 * This file is used from processing http requests from the frontend
 */

/**
 * I put this here because this will be returned as json for the api
 */
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

/**
 * Used to store the config.json file and some database related stuff for easy access
 *
 */
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

/**
 * Not all events are processed at once and we do not want to miss out on events, so put them in a queque and use FIFO to process.
 */
var EventsQueue = make([]nostr.Event, 0)

// var ptagsQueue = make([]string, 0)
var syncHash string = ""

/*
 * Please see https://github.com/mattn/algia/blob/main/main.go for the code i shamelessly copied
 *
 * Fire off calls to relays for getting new posts, user metadata etc. Each relay is operated in it's own thread
 * The f function is used to process the data we get from the relays.
 *
 * It just makes sure all available relays are called
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
			f(relay) // Custom function call that takes nostr.Relay as an argument. The function f will probally be an anonymous function
			relay.Close()
		}(&wg, v)
	}
	wg.Wait()
}

/**
 * Send a request over a websocket to get new events (notes) and after processing the events
 * try to get all the usernames metadata from who posted the note.
 */
func (cfg *Config) getEvents(filter nostr.Filter) {
	log.Println("Get Event data from relays")
	var m sync.Map
	cfg.Do(func(relay *nostr.Relay) {
		evs, err := relay.QuerySync(context.Background(), filter)
		if err != nil {
			return
		}
		/**
		 * Deduplicate
		 * Make sure we only have 1 copy of the event even when we have multiple relays that have this event stored.
		 */
		for _, ev := range evs {
			if _, ok := m.Load(ev.ID); !ok {
				m.LoadOrStore(ev.ID, ev)
			}
		}
	})

	/**
	 * Turn the sync map into an array we can process
	 */
	var evs []*nostr.Event
	m.Range(func(k, v any) bool {
		log.Println(k)
		evs = append(evs, v.(*nostr.Event))
		return true
	})

	/**
	 * Array of all the pubkeys in the event also the #p tags
	 */
	var pubkeys = make([]string, 0)
	pubkeys = cfg.Storage.SaveEvents(evs)
	for i, pubkey := range pubkeys {
		log.Println(i, pubkey)
	}

	// Last but not least, try to get the user metadata
	defer cfg.updateProfiles(pubkeys)

	defer func() {
		log.Println("Done receiving and closed ralay connections")
	}()
}

/**
 * Before we try to get events, first get the last timestamp so we do not query all the events all the time but only the lastests.
 * We do not want to spam the relays when we just synced, so wait 60 seconds before we accept a new sync
 */
func (cfg *Config) getEventData() {
	var createdAt int64
	var createdAtOffset int64 = time.Now().Unix() - 60

	//row := cfg.Storage.Db.QueryRow("SELECT MAX(created_at) as MaxCreated FROM events")
	//row.Scan(&createdAt)
	createdAt = cfg.Storage.getLastTimeStamp()

	log.Println(createdAt)
	if createdAt < 1 {
		createdAt = createdAtOffset
	}
	if createdAt > createdAtOffset {
		log.Printf("Time lapse is to short for getting new data %d %d", createdAt, createdAtOffset)
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

/**
 * Get the metadata of a bunch of Pubkeys and store them.
 */
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

/**
 * Put a user on the naugthy list.
 */
func (cfg *Config) blockPubkey(user *BlockPubkey) {
	err := cfg.Storage.BlockPubkey(user.Pubkey)
	if err != nil {
		log.Println(err)
	}
}

/**
 * Get the content of config.json file
 */
func loadConfig() (*Config, error) {
	var cfg Config

	content, err := os.ReadFile("./config.json")
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

/**
 * Process all the http calls
 */
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

	/*
	 * Get events that already are stored in the database
	 * This will not SYNC the local database with that of the relays.
	 */
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

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)

		if err != nil {
			log.Println(err)
		}
		json.NewEncoder(w).Encode(&pagination)
	})

	/**
	 * This will sync the local database with that of the relays (Only public events and not channels and such)
	 */
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

	/**
	 * Put a user on the naughty list
	 */
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

	/**
	 * Put a user on the follow list
	 * This is all local and will not send an event for followlist
	 */
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

	/**
	 * Find an event based on event id. This can be a reply
	 */
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
			filter := nostr.Filter{
				IDs:   []string{j.ID},
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

	/**
	 * Sometimes it is nice to see pictures in the post and not just a link
	 */
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
