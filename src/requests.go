package main

import (
	"amavis442/nostr-reader/database"
	nostrWrapper "amavis442/nostr-reader/nostr/wrapper"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

type Requests struct {
	Cfg   *Config
	Db    *database.Storage
	Nostr *nostrWrapper.NostrWrapper
}

/**
 * I put this here because this will be returned as json for the api
 */
type Pubkey struct {
	Pubkey string `json:"pubkey"`
}

type BookMark struct {
	EventId string `json:"event_id"`
}

/**
 * Not all events are processed at once and we do not want to miss out on events, so put them in a queque and use FIFO to process.
 */
//var EventsQueue = make([]nostr.Event, 0)

// var ptagsQueue = make([]string, 0)
var syncHash string = ""

type Page struct {
	Page    int
	Limit   int
	Since   int
	Maxid   int
	Renew   bool
	Context string
}

type Response struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}
type ResponseProfile struct {
	Result map[string]string `json:"result"`
}

type ResponseRelay struct {
	Response
	Relays []database.Relay `json:"relays"`
}

/**
* The API requests
 */
func (req *Requests) GetNotes(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	var p Page
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	pagination := database.Pagination{}
	pagination.SetLimit(p.Limit)
	pagination.SetCurrentPage(p.Page)
	pagination.SetSince(p.Since)

	pagination.SetRenew(p.Renew)
	pagination.SetMaxId(p.Maxid)

	err = req.Db.GetPagination(ctx, &pagination, database.Options{Follow: false, BookMark: false})

	if err != nil {
		log.Println(err)
	}
	err = json.NewEncoder(w).Encode(&pagination)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetInbox(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var p Page
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	pagination := database.Pagination{}
	pagination.SetLimit(p.Limit)
	pagination.SetCurrentPage(p.Page)
	pagination.SetSince(p.Since)
	err = req.Db.GetInbox(ctx, &pagination, req.Cfg.PubKey)

	if err != nil {
		log.Println(err)
	}
	err = json.NewEncoder(w).Encode(&pagination)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) StartSync(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	response := &Response{}
	response.Status = "ok"
	syncHash = fmt.Sprint(time.Now().Unix())
	response.Message = syncHash

	//EventsQueue = EventsQueue[:0]
	createdAt := req.Db.GetLastTimeStamp(ctx)

	filter := req.Nostr.GetEventData(createdAt, true)
	evs := req.Nostr.GetEvents(ctx, filter)
	var pubkeys = make([]string, 0)
	var err error
	pubkeys, err = req.Db.SaveEvents(ctx, evs)
	if err != nil {
		response.Status = "error"
		response.Message = err.Error()
	}

	// Todo build check for ttl so user data is not refreshed every time.
	var tresholdTime int64 = time.Now().Unix() - 60*60*24

	pubkeys, _ = req.Db.CheckProfiles(ctx, pubkeys, tresholdTime)
	// Last but not least, try to get the user metadata
	req.Nostr.UpdateProfiles(ctx, pubkeys)
	req.Db.SaveProfiles(ctx, evs)

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) SyncNote(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	type Request struct {
		ID string
	}
	var j Request
	err := json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	log.Println("Sync event with Id: ", j.ID)
	var tagMap nostr.TagMap = make(nostr.TagMap, 0)
	tagMap["e"] = []string{j.ID}
	filter := nostr.Filter{
		Tags:  tagMap,
		Limit: 1,
	}

	evs := req.Nostr.GetEvents(ctx, filter)

	req.Db.SaveEvents(ctx, evs)

	ev, _ := req.Db.FindEvent(ctx, j.ID)

	log.Println("Need to get it", j.ID, filter)

	syncHash = fmt.Sprint(time.Now().Unix())

	response := &Response{}
	response.Status = "ok"
	response.Message = syncHash
	response.Data = ev
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) BlockUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var user Pubkey
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		log.Println(err)
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	req.Db.CreateBlock(ctx, user.Pubkey)

	response := &Response{}
	response.Status = "ok"
	response.Message = "Blocked pubkey: " + user.Pubkey
	response.Data = user.Pubkey

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) Follow(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var user Pubkey
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		log.Println(err)
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	if user.Pubkey[0:4] == "npub" {
		prefix, value, err := nip19.Decode(user.Pubkey)
		if err != nil {
			log.Println(prefix, value, err)
			panic(err)
		}
		user.Pubkey = value.(string)
		//log.Println(prefix, value, err)
	}

	err = req.Db.CreateFollow(ctx, user.Pubkey)
	response := &Response{}
	response.Status = "ok"
	response.Message = "Follow pubkey: " + user.Pubkey
	response.Data = user.Pubkey
	if err != nil {
		response.Status = "error"
		response.Message = err.Error()
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) Unfollow(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var user Pubkey
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		log.Println(err)
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	err = req.Db.RemoveFollow(ctx, user.Pubkey)

	response := &Response{}
	response.Status = "ok"
	response.Message = "Unfollow pubkey: " + user.Pubkey
	response.Data = user.Pubkey
	if err != nil {
		response.Status = "error"
		response.Message = err.Error()
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetFollowedNotes(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	var p Page
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	pagination := database.Pagination{}
	pagination.SetLimit(p.Limit)
	pagination.SetCurrentPage(p.Page)
	pagination.SetSince(p.Since)
	pagination.SetRenew(p.Renew)
	pagination.SetMaxId(p.Maxid)
	err = req.Db.GetPagination(ctx, &pagination, database.Options{Follow: true, BookMark: false})

	if err != nil {
		log.Println(err)
	}
	err = json.NewEncoder(w).Encode(&pagination)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetFollowedProfiles(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	profiles := req.Db.GetFollowedProfiles(ctx)

	response := &Response{}
	response.Data = profiles
	response.Status = "ok"
	response.Message = "Profiles"

	err := json.NewEncoder(w).Encode(&response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) AddBookMark(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var j BookMark
	err := json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		log.Println(err)
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	err = req.Db.CreateBookMark(ctx, j.EventId)

	response := &Response{}
	response.Status = "ok"
	response.Message = "Bookmark"
	if err != nil {
		response.Status = "error"
		response.Message = err.Error()
	}

	response.Data = j.EventId

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) RemoveBookMark(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var j BookMark
	err := json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		log.Println(err)
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	err = req.Db.RemoveBookMark(ctx, j.EventId)

	response := &Response{}
	response.Status = "ok"
	response.Message = "Bookmark"

	response.Status = "ok"
	response.Message = "Remove bookmark"
	if err != nil {
		response.Status = "error"
		response.Message = err.Error()
	}

	response.Data = j.EventId

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) AddRelay(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var j database.Relay
	err := json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		log.Println(err)
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	response := &Response{}
	response.Status = "ok"
	response.Message = "Relay added"

	err = req.Db.CreateRelay(ctx, &j)
	if err != nil {
		response.Status = "error"
		response.Message = err.Error()
	}

	relays := req.Db.GetRelays(ctx)
	response.Data = relays

	UpdateRelays(&req.Nostr.Cfg, relays)

	err = json.NewEncoder(w).Encode(&response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) RemoveRelay(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var j database.Relay
	err := json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		log.Println(err)
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	response := &Response{}
	response.Status = "ok"
	response.Message = "Remove relay: " + j.Url
	err = req.Db.RemoveRelay(ctx, j.Url)
	if err != nil {
		response.Status = "error"
		response.Message = err.Error()
	}

	relays := req.Db.GetRelays(ctx)
	response.Data = relays

	UpdateRelays(&req.Nostr.Cfg, relays)

	err = json.NewEncoder(w).Encode(&response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetRelays(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	response := &Response{}
	response.Status = "ok"
	response.Message = "Relays"
	relays := req.Db.GetRelays(ctx)
	response.Data = relays

	err := json.NewEncoder(w).Encode(&response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetBookMarked(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	var p Page
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	pagination := database.Pagination{}
	pagination.SetLimit(p.Limit)
	pagination.SetCurrentPage(p.Page)
	pagination.SetSince(p.Since)
	pagination.SetRenew(p.Renew)
	pagination.SetMaxId(p.Maxid)
	err = req.Db.GetPagination(ctx, &pagination, database.Options{Follow: false, BookMark: true})

	if err != nil {
		log.Println(err)
	}
	err = json.NewEncoder(w).Encode(&pagination)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetNewNotesCount(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var p Page
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	options := database.Options{
		Follow:   false,
		BookMark: false,
	}

	if p.Context == "follow" {
		options.Follow = true
	}
	if p.Context == "bookmark" {
		options.BookMark = true
	}

	response := &Response{}
	response.Status = "ok"
	response.Message = "new notes count"
	count, err := req.Db.GetNewNotesCount(ctx, p.Maxid, options)

	response.Data = fmt.Sprintf("%d", count)

	if err != nil {
		response.Status = "error"
		response.Message = err.Error()
		response.Data = "0"
	}

	err = json.NewEncoder(w).Encode(&response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetLastSeenID(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	maxid, err := req.Db.GetLastSeenID(ctx)

	response := &Response{}
	response.Status = "ok"
	response.Message = "new notes count"
	response.Data = maxid

	if err != nil {
		response.Status = "error"
		response.Message = err.Error()
		response.Data = 0
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) SearchEvent(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	type Request struct {
		ID string
	}
	var j Request
	err := json.NewDecoder(r.Body).Decode(&j)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	log.Println("Searching event with Id: ", j.ID)
	ev, _ := req.Db.FindEvent(ctx, j.ID)
	if ev.Event.ID == "" {
		filter := nostr.Filter{
			IDs:   []string{j.ID},
			Limit: 1,
		}

		evs := req.Nostr.GetEvents(ctx, filter)

		req.Db.SaveEvents(ctx, evs)

		log.Println("Need to get it", j.ID, filter)
	}
	ev, _ = req.Db.FindEvent(ctx, j.ID)

	response := &Response{}
	response.Status = "ok"
	response.Message = "Result find for event: " + j.ID
	response.Data = ev

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}

/**
 * Need an easy way to cancel this request when a new nextpage or refreshpage comes in
 */
func (req *Requests) PreviewLink(w http.ResponseWriter, r *http.Request) {
	var mu sync.Mutex

	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	type Url struct {
		Url string
	}
	var url Url
	err := json.NewDecoder(r.Body).Decode(&url)
	if err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")

	t := strings.TrimSpace(url.Url)
	s := strings.Split(t, "\n")

	mu.Lock()
	result, _ := URLPreview(ctx, s[0])
	mu.Unlock()

	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) Publish(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
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

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")

	log.Println("Msg to publish: ", msg.Msg)
	var postEv nostr.Event
	if msg.Event_id == "" {
		postEv, _ = req.Nostr.DoPost(ctx, msg.Msg)

		req.Db.SaveEvents(ctx, []*nostr.Event{&postEv})
	}

	if msg.Event_id != "" {
		replyEv, _ := req.Db.FindRawEvent(ctx, msg.Event_id)
		postEv, _ = req.Nostr.DoReply(ctx, msg.Msg, *replyEv)

		req.Db.SaveEvents(ctx, []*nostr.Event{&postEv})
	}

	response := &Response{}
	response.Status = "ok"
	response.Message = msg.Msg

	jsonPostEv, _ := json.Marshal(postEv)
	response.Data = string(jsonPostEv)

	if err != nil {
		log.Println(err)
		response.Status = "error"
		response.Message = err.Error()
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetMetaData(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	response := &Response{}
	response.Status = "ok"
	response.Message = "Metadata"

	event, err := req.Nostr.GetMetaData(ctx)
	response.Data = event
	if err != nil {
		response.Status = "error"
		response.Message = err.Error()
	}

	req.Db.SaveProfiles(ctx, []*nostr.Event{&event})

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) SetMetaData(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var user nostrWrapper.Profile
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		log.Println(err)
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	user.Pubkey = req.Cfg.PubKey
	err = req.Nostr.DoPublishMetaData(ctx, &user)

	response := &Response{}
	response.Status = "ok"
	response.Message = "Set metadata"
	response.Data = user

	if err != nil {
		response.Status = "error"
		response.Message = err.Error()
	}

	err = json.NewEncoder(w).Encode(user)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetProfile(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := context.Background()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	profile, err := req.Db.FindProfile(ctx, req.Cfg.PubKey)

	response := &Response{}
	response.Status = "ok"
	response.Message = "Profile"
	response.Data = profile

	if err != nil {
		response.Status = "error"
		response.Message = err.Error()
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic(err)
	}
}
