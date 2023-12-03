package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	nostrHandler "github.com/nbd-wtf/go-nostr"
)

type Requests struct {
	Cfg   *Config
	Nostr *Nostr
}

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

/**
 * Not all events are processed at once and we do not want to miss out on events, so put them in a queque and use FIFO to process.
 */
var EventsQueue = make([]nostrHandler.Event, 0)

// var ptagsQueue = make([]string, 0)
var syncHash string = ""

/**
* The API requests
 */
func (req *Requests) getRoot(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	type Page struct {
		Page  int
		Limit int
		Since int
	}
	var p Page
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		panic(err)
	}

	pagination := Pagination{}
	pagination.SetLimit(p.Limit)
	pagination.SetCurrentPage(p.Page)
	pagination.SetSince(p.Since)
	err = req.Cfg.Storage.GetEventPagination(&pagination)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	if err != nil {
		log.Println(err)
	}
	json.NewEncoder(w).Encode(&pagination)
}

func (req *Requests) StartSync(w http.ResponseWriter, r *http.Request) {

	EventsQueue = EventsQueue[:0]
	req.Nostr.getEventData()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	syncHash = fmt.Sprint(time.Now().Unix())

	test := make(map[string]string)
	test["status"] = "ok"
	test["message"] = syncHash
	json.NewEncoder(w).Encode(test)
}

func (req *Requests) BlockUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var j BlockPubkey
	err := json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		panic(err)
	}

	req.Nostr.blockPubkey(&j)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)
	test := map[string]string{}
	test["status"] = "ok"
	test["blocked"] = j.Pubkey
	json.NewEncoder(w).Encode(test)
}

func (req *Requests) FollowUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var user FollowPubkey
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		panic(err)
	}

	err = req.Cfg.Storage.FollowPubkey(user.Pubkey)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)
	test := map[string]string{}
	test["status"] = "ok"
	test["msg"] = ""
	if err != nil {
		test["status"] = "error"
		test["msg"] = err.Error()
	}

	test["followed"] = user.Pubkey
	json.NewEncoder(w).Encode(test)
}

func (req *Requests) SearchEvent(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	type Request struct {
		ID string
	}
	var j Request
	err := json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		panic(err)
	}
	log.Println("Searching event with Id: ", j.ID)
	ev := req.Cfg.Storage.FindEvent(j.ID)
	if ev.EventID == "" {
		req.Nostr.GetEventById(j.ID)
	}
	ev = req.Cfg.Storage.FindEvent(j.ID)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ev)
}

func (req *Requests) PreviewLink(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	type Url struct {
		Url string
	}
	var url Url
	err := json.NewDecoder(r.Body).Decode(&url)
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
}

func (req *Requests) Publish(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	type Msg struct {
		Msg string
	}
	var msg Msg
	err := json.NewDecoder(r.Body).Decode(&msg)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")

	w.WriteHeader(http.StatusOK)
	log.Println("Msg to publish: ", msg.Msg)

	req.Nostr.Post(msg.Msg)

	test := map[string]string{}
	test["status"] = "ok"
	test["msg"] = msg.Msg
	json.NewEncoder(w).Encode(test)
}
