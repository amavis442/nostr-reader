package main

import (
	"context"
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

type Page struct {
	Page  int
	Limit int
	Since int
}

/**
* The API requests
 */
func (req *Requests) getRoot(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var p Page
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		panic(err)
	}

	pagination := Pagination{}
	pagination.SetLimit(p.Limit)
	pagination.SetCurrentPage(p.Page)
	pagination.SetSince(p.Since)
	err = req.Cfg.Storage.GetEventPagination(ctx, &pagination, false)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	if err != nil {
		log.Println(err)
	}
	err = json.NewEncoder(w).Encode(&pagination)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) StartSync(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	EventsQueue = EventsQueue[:0]
	req.Nostr.getEventData(ctx)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	syncHash = fmt.Sprint(time.Now().Unix())

	test := make(map[string]string)
	test["status"] = "ok"
	test["message"] = syncHash
	err := json.NewEncoder(w).Encode(test)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) SyncNote(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	type Request struct {
		ID string
	}
	var j Request
	err := json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		panic(err)
	}
	log.Println("Sync event with Id: ", j.ID)
	var tagMap nostrHandler.TagMap = make(nostrHandler.TagMap, 0)
	tagMap["e"] = []string{j.ID}
	filter := nostrHandler.Filter{
		Tags:  tagMap,
		Limit: 1,
	}

	req.Nostr.GetEvents(ctx, filter, false)
	ev, err := req.Cfg.Storage.FindEvent(ctx, j.ID)

	log.Println("Need to get it", j.ID, filter)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	syncHash = fmt.Sprint(time.Now().Unix())

	type Result struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    Event  `json:"data"`
	}
	var test = Result{}
	//test := make(map[string]string)
	test.Status = "ok"
	test.Message = syncHash
	test.Data = ev
	err = json.NewEncoder(w).Encode(test)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) BlockUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var j BlockPubkey
	err := json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		log.Println(err)
		panic(err)
	}

	req.Nostr.blockPubkey(ctx, &j)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)
	test := map[string]string{}
	test["status"] = "ok"
	test["blocked"] = j.Pubkey
	err = json.NewEncoder(w).Encode(test)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) FollowUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user FollowPubkey
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		log.Println(err)
		panic(err)
	}

	err = req.Nostr.FollowPubkey(ctx, &user)

	fmt.Println("Follow user: ", user.Pubkey)
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
	err = json.NewEncoder(w).Encode(test)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) UnfollowUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user FollowPubkey
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		log.Println(err)
		panic(err)
	}

	err = req.Nostr.UnfollowPubkey(ctx, &user)

	fmt.Println("Unfollow user: ", user.Pubkey)
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
	err = json.NewEncoder(w).Encode(test)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) FollowUserNotes(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var p Page
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		panic(err)
	}

	pagination := Pagination{}
	pagination.SetLimit(p.Limit)
	pagination.SetCurrentPage(p.Page)
	pagination.SetSince(p.Since)
	err = req.Cfg.Storage.GetEventPagination(ctx, &pagination, true)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	if err != nil {
		log.Println(err)
	}
	err = json.NewEncoder(w).Encode(&pagination)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) SearchEvent(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	type Request struct {
		ID string
	}
	var j Request
	err := json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		panic(err)
	}
	log.Println("Searching event with Id: ", j.ID)
	ev, _ := req.Cfg.Storage.FindEvent(ctx, j.ID)
	if ev.Event.ID == "" {
		filter := nostrHandler.Filter{
			IDs:   []string{j.ID},
			Limit: 1,
		}

		req.Nostr.GetEvents(ctx, filter, false)

		log.Println("Need to get it", j.ID, filter)
	}
	ev, _ = req.Cfg.Storage.FindEvent(ctx, j.ID)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(ev)
	if err != nil {
		panic(err)
	}
}

/**
 * Need an easy way to cancel this request when a new nextpage or refreshpage comes in
 */
func (req *Requests) PreviewLink(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

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

	result, err := URLPreview(ctx, s[0])
	if err != nil {
		log.Println(err)

	}
	log.Println("Preview result: ", result)
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) Publish(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	type Msg struct {
		Msg      string
		Event_id string // If it is a reply
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
	postEv, _ := req.Nostr.Post(ctx, msg.Msg, msg.Event_id)
	test := map[string]string{}

	test["status"] = "ok"
	test["msg"] = msg.Msg
	test["reply_to_event_id"] = msg.Event_id
	jsonPostEv, _ := json.Marshal(postEv)
	test["post"] = string(jsonPostEv)

	if err != nil {
		log.Println(err)
		test["status"] = "error"
		test["msg"] = err.Error()
	}

	err = json.NewEncoder(w).Encode(test)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetMetaData(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	event, _ := req.Nostr.GetMetaData(ctx)

	req.Cfg.Storage.SaveProfiles(ctx, []*nostrHandler.Event{&event})

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(event)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) SetMetaData(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var user UserProfile
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		log.Println(err)
		panic(err)
	}
	user.Pubkey = req.Cfg.Pubkey
	_ = req.Nostr.SetMetaData(ctx, &user)

	//fmt.Println("Follow user: ", user.Pubkey)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(user)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetProfile(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	profile, _ := req.Cfg.Storage.FindProfile(ctx, req.Cfg.Pubkey)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(profile)
	if err != nil {
		panic(err)
	}
}
