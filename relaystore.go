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
	"strings"
	"time"

	"github.com/lib/pq"
	_ "github.com/lib/pq"

	"github.com/nbd-wtf/go-nostr"
)

const (
	host     = "localhost"
	port     = 5432
	user     = "nostr"
	password = "nostr"
	dbname   = "nostr"
)

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
	Name      string   `json:"name"`
	About     string   `json:"about"`
	Picture   string   `json:"picture"`
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

type Config struct {
	Database *DbConfig
	Relays   []string
	Pubkey   string
	Npub     string
	Pk       string
	Nsec     string
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

type User struct {
	Name    string `json:"name"`
	About   string `json:"about"`
	Picture string `json:"picture"`
}

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

func receiveAndProcessPoolEvents(ctx context.Context, db *sql.DB, pool *nostr.SimplePool, filters nostr.Filters) {

	var qry = `INSERT INTO "events" ("id", "pubkey", "kind", "created_at", "content" , "tags_full" , "sig" , "raw" , "ptags" , "etags") 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT (id) DO NOTHING;`

	var pubkeys = make([]string, 0)

	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	stmt, err := tx.Prepare(qry)
	if err != nil {
		panic(err)
	}
	defer stmt.Close() // Prepared statements take up server resources and should be closed after use.

	ctxChild, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	ptags, etags := make([]string, 0), make([]string, 0)
	log.Println("Receiving from channel")
	for ev := range pool.SubMany(ctxChild, settings.Relays, filters) {
		log.Println("Event ID: ", ev.ID)

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

		//etagsString := strings.Join(etags, "\n")
		//ptagsString := strings.Join(ptags, "\n")

		tagJson, err := json.Marshal(ev.Tags)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Add to transaction")
		if _, err := stmt.Exec(ev.ID, ev.PubKey, ev.Kind, ev.CreatedAt, ev.Content, string(tagJson), ev.Sig, ev.String(), pq.Array(ptags), pq.Array(etags)); err != nil {
			panic(err)
		}
		log.Println("Waiting for next event")
	}

	log.Println("Ready to save events")
	if err := tx.Commit(); err != nil {
		panic(err)
	}

	go updateUsers(pubkeys, db, pool)

	defer func() {
		log.Println("Done receiving and closing context")
		//sub.Unsub()
		//processQueue(db)
		//relay.Close()
		//wg.Done()
	}()
}

func processQueue(db *sql.DB) {
	var qry = `INSERT OR IGNORE INTO events (id, pubkey, kind, created_at, content,tags_full, sig, raw, p_tags, e_tags) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	ptags, etags := make([]string, 0), make([]string, 0)

	defer func() {
		log.Println("Done processing")
	}()

	for _, ev := range EventsQueue {
		log.Println("Processing Queue")
		log.Println(ev.String())

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
	}
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

func getRelayData(db *sql.DB, ctxParent context.Context, relay *nostr.Relay) { //}, wg *sync.WaitGroup) {
	var createdAt int64
	row := db.QueryRow("SELECT MAX(created_at) as MaxCreated FROM events")
	row.Scan(&createdAt)
	log.Println(createdAt)
	if createdAt < 1 {
		createdAt = time.Now().Unix() - 60
	}
	if createdAt > time.Now().Unix()-60 {
		log.Printf("Time lapse is to short for getting new data %d %d", createdAt, time.Now().Unix()-60)
		return
	}

	ctx, cancel := context.WithTimeout(ctxParent, 15*time.Second)
	defer cancel()
	sub, err := relay.Subscribe(ctx, getFilters(createdAt))
	if err != nil {
		panic(err)
	}

	receiveEvents(sub)

	defer func() {
		log.Println("Closing shop")
		sub.Unsub()
		processQueue(db)
		//relay.Close()
		//wg.Done()
	}()
}

func getPoolData(db *sql.DB, ctxParent context.Context, pool *nostr.SimplePool) {
	var createdAt int64
	row := db.QueryRow("SELECT MAX(created_at) as MaxCreated FROM events")
	row.Scan(&createdAt)
	log.Println(createdAt)
	if createdAt < 1 {
		createdAt = time.Now().Unix() - 60
	}
	if createdAt > time.Now().Unix()-60 {
		log.Printf("Time lapse is to short for getting new data %d %d", createdAt, time.Now().Unix()-60)
		return
	}

	ctx, cancel := context.WithTimeout(ctxParent, 15*time.Second)
	defer cancel()
	receiveAndProcessPoolEvents(ctx, db, pool, getFilters(createdAt))

	defer func() {
		log.Println("Closing shop")
	}()
}

func updateUsers(pubkeys []string, db *sql.DB, pool *nostr.SimplePool) {
	var err error
	filters := []nostr.Filter{{
		Kinds:   []int{nostr.KindSetMetadata},
		Authors: pubkeys,
	}}

	var qry = `INSERT INTO "users" ("pubkey", "name","about", "picture", "raw", "created_at") 
		VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (pubkey) DO NOTHING;`

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var tx *sql.Tx
	tx, err = db.Begin()
	if err != nil {
		panic(err)
	}
	for ev := range pool.SubManyEose(ctx, settings.Relays, filters) {
		var data User
		err = json.Unmarshal([]byte(ev.Content), &data)
		if err != nil {
			panic(err)
		}

		_, err = tx.Exec(qry, ev.PubKey, data.Name, data.About, data.Picture, ev.String(), time.Now().Unix())
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.Fatalf("update users: unable to rollback: %v", rollbackErr)
			}
			log.Fatal(err)
			ctx.Done()
		}
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}

	log.Println("Done for users")
}

func getLast10(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	tx, err := myDb.Begin()
	if err != nil {
		panic(err)
	}

	rows, err := tx.Query(`SELECT e.id, e.pubkey, e.kind, e.created_at, e.content, e.tags_full, e.etags, e.ptags, e.sig, u.name, u.about , u.picture
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
		if err := rows.Scan(&event.ID, &event.Pubkey, &event.Kind, &event.CreatedAt, &event.Content, &event.Tags_full, pq.Array(&event.Etags), pq.Array(&event.Ptags), &event.Sig, &name, &about, &picture); err != nil {
			log.Fatal(err)
		}
		if name.Valid {
			event.Name = name.String
		} else {
			event.Name = event.Pubkey
		}
		if about.Valid {
			event.About = about.String
		}
		if picture.Valid {
			event.Picture = picture.String
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

var settings Config

func readConfig() error {
	content, err := ioutil.ReadFile("./config.json")
	if err != nil {
		fmt.Println("Done", err)
		log.Fatal("Error when opening file: ", err)
		return err
	}

	err = json.Unmarshal(content, &settings)
	if err != nil {
		log.Fatal("Error during Unmarshal(): ", err)
		return err
	}

	//log.Println("Content nieuw", *settings)
	// Let's print the unmarshalled data!
	log.Printf("dbName: %s\n", settings.Database.Dbname)
	log.Printf("Pubkey: %s\n", settings.Pubkey)
	return nil
}

func main() {
	if err := readConfig(); err != nil {
		panic(err)
	}

	// connection string
	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", settings.Database.Host, settings.Database.Port, settings.Database.User, settings.Database.Password, settings.Database.Dbname)

	// open database
	db, err := sql.Open("postgres", psqlconn)
	CheckError(err)

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

	//var wg sync.WaitGroup
	//wg.Add(1)
	//ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) //Keep it open for 1 minute and then close
	//defer cancel()
	//go getRelayData(db, ctx)
	//wg.Wait()

	//ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second) //Keep it open for 1 minute and then close
	//defer cancel()
	ctx := context.Background()
	pool := nostr.NewSimplePool(context.Background())

	// Windows may be missing this
	mime.AddExtensionType(".js", "application/javascript")
	http.Handle("/api/follow", http.HandlerFunc(getLast10))
	http.HandleFunc("/api/getnext", func(w http.ResponseWriter, r *http.Request) {

		EventsQueue = EventsQueue[:0]
		go getPoolData(db, ctx, pool)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)
		test := []string{}
		test = append(test, "Hello")
		test = append(test, "World")
		json.NewEncoder(w).Encode(test)
	})

	http.HandleFunc("/api/updateusers", func(w http.ResponseWriter, r *http.Request) {

		rows, err := myDb.Query("SELECT pubkey FROM events WHERE kind = 1 ORDER BY created_at DESC LIMIT 30")
		if err != nil {
			log.Fatal(err)
			return
		}
		defer rows.Close()
		var pubkeys []string
		var pubkey string
		for rows.Next() {
			rows.Scan(&pubkey)
			pubkeys = append(pubkeys, pubkey)
		}
		go updateUsers(pubkeys, db, pool)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)
		test := []string{}
		test = append(test, "Hello")
		test = append(test, "Users")
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

	http.Handle("/", http.FileServer(http.Dir("web/nostr-reader/dist")))

	log.Fatal(http.ListenAndServe(":8080", nil))

}
