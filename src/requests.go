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

/**
* The API requests
 */
func (req *Requests) GetNotes(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var p Page
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		panic(err)
	}

	pagination := database.Pagination{}
	pagination.SetLimit(p.Limit)
	pagination.SetCurrentPage(p.Page)
	pagination.SetSince(p.Since)

	pagination.SetRenew(p.Renew)
	pagination.SetMaxId(p.Maxid)

	err = req.Db.GetPagination(ctx, &pagination, database.Options{Follow: false, BookMark: false})

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

func (req *Requests) GetInbox(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var p Page
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		panic(err)
	}

	pagination := database.Pagination{}
	pagination.SetLimit(p.Limit)
	pagination.SetCurrentPage(p.Page)
	pagination.SetSince(p.Since)
	err = req.Db.GetInbox(ctx, &pagination, req.Cfg.PubKey)

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

	response := make(map[string]string)
	response["status"] = "ok"
	syncHash = fmt.Sprint(time.Now().Unix())
	response["message"] = syncHash

	//EventsQueue = EventsQueue[:0]
	createdAt := req.Db.GetLastTimeStamp(ctx)

	filter := req.Nostr.GetEventData(createdAt, true)
	evs := req.Nostr.GetEvents(filter)
	var pubkeys = make([]string, 0)
	var err error
	pubkeys, err = req.Db.SaveEvents(ctx, evs)
	if err != nil {
		response["status"] = "error"
		response["message"] = err.Error()
	}

	// Todo build check for ttl so user data is not refreshed every time.
	var tresholdTime int64 = time.Now().Unix() - 60*60*24

	pubkeys, _ = req.Db.CheckProfiles(ctx, pubkeys, tresholdTime)
	// Last but not least, try to get the user metadata
	req.Nostr.UpdateProfiles(pubkeys)
	req.Db.SaveProfiles(ctx, evs)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(response)
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
	var tagMap nostr.TagMap = make(nostr.TagMap, 0)
	tagMap["e"] = []string{j.ID}
	filter := nostr.Filter{
		Tags:  tagMap,
		Limit: 1,
	}

	evs := req.Nostr.GetEvents(filter)

	req.Db.SaveEvents(ctx, evs)

	ev, _ := req.Db.FindEvent(ctx, j.ID)

	log.Println("Need to get it", j.ID, filter)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	syncHash = fmt.Sprint(time.Now().Unix())

	type Result struct {
		Status  string         `json:"status"`
		Message string         `json:"message"`
		Data    database.Event `json:"data"`
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

	var user Pubkey
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		log.Println(err)
		panic(err)
	}

	req.Db.CreateBlock(ctx, user.Pubkey)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)
	test := map[string]string{}
	test["status"] = "ok"
	test["blocked"] = user.Pubkey
	err = json.NewEncoder(w).Encode(test)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) Follow(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user Pubkey
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		log.Println(err)
		panic(err)
	}

	err = req.Db.CreateFollow(ctx, user.Pubkey)

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

func (req *Requests) Unfollow(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user Pubkey
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		log.Println(err)
		panic(err)
	}

	err = req.Db.RemoveFollow(ctx, user.Pubkey)

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

func (req *Requests) GetFollowed(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var p Page
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		panic(err)
	}

	pagination := database.Pagination{}
	pagination.SetLimit(p.Limit)
	pagination.SetCurrentPage(p.Page)
	pagination.SetSince(p.Since)
	pagination.SetRenew(p.Renew)
	pagination.SetMaxId(p.Maxid)
	err = req.Db.GetPagination(ctx, &pagination, database.Options{Follow: true, BookMark: false})

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

func (req *Requests) AddBookMark(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	result := map[string]string{}
	result["status"] = "ok"
	result["msg"] = "Bookmark"
	if err != nil {
		result["status"] = "error"
		result["msg"] = err.Error()
	}

	result["data"] = j.EventId
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) RemoveBookMark(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	fmt.Println("Remove bookmark: ", j.EventId)
	result := map[string]string{}
	result["status"] = "ok"
	result["msg"] = "Remove bookmark"
	if err != nil {
		result["status"] = "error"
		result["msg"] = err.Error()
	}

	result["data"] = j.EventId
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		panic(err)
	}
}

type RetVal struct {
	Result map[string]string `json:"result"`
	Relays []database.Relay  `json:"relays"`
}

func (req *Requests) AddRelay(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	relay, err := req.Db.CreateRelay(ctx, j.Url, j.Write, j.Read, j.Search)

	result := map[string]string{}
	result["status"] = "ok"
	result["msg"] = "Relay added"
	if err != nil {
		result["status"] = "error"
		result["msg"] = err.Error()
	}
	resultJson, _ := json.Marshal(relay)
	result["data"] = string(resultJson)

	relays := req.Db.GetRelays(ctx)
	retval := &RetVal{}
	if len(relays) > 0 {
		retval.Relays = relays
	}
	UpdateRelays(&req.Nostr.Cfg, relays)

	retval.Result = result

	err = json.NewEncoder(w).Encode(&retval)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) RemoveRelay(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

	err = req.Db.RemoveRelay(ctx, j.Url)

	fmt.Println("Remove relay: ", j.Url)
	result := map[string]string{}
	result["status"] = "ok"
	result["msg"] = "Remove relay"
	if err != nil {
		result["status"] = "error"
		result["msg"] = err.Error()
	}

	relays := req.Db.GetRelays(ctx)

	retval := &RetVal{}
	if len(relays) > 0 {
		retval.Relays = relays
	}
	UpdateRelays(&req.Nostr.Cfg, relays)

	result["data"] = j.Url
	retval.Result = result

	err = json.NewEncoder(w).Encode(&retval)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetRelays(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	relays := req.Db.GetRelays(ctx)

	retval := &RetVal{}
	if len(relays) > 0 {
		retval.Relays = relays
	}

	result := map[string]string{}
	result["status"] = "ok"
	result["msg"] = "Relays"

	retval.Result = result

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	err := json.NewEncoder(w).Encode(&retval)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetBookMarked(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var p Page
	err := json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		panic(err)
	}

	pagination := database.Pagination{}
	pagination.SetLimit(p.Limit)
	pagination.SetCurrentPage(p.Page)
	pagination.SetSince(p.Since)
	pagination.SetRenew(p.Renew)
	pagination.SetMaxId(p.Maxid)
	err = req.Db.GetPagination(ctx, &pagination, database.Options{Follow: false, BookMark: true})

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

func (req *Requests) GetNewNotesCount(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	count, err := req.Db.GetNewNotesCount(ctx, p.Maxid, options)
	result := map[string]string{}
	result["status"] = "ok"
	result["msg"] = "new notes count"
	result["data"] = fmt.Sprintf("%d", count)

	if err != nil {
		result["status"] = "error"
		result["msg"] = err.Error()
		result["data"] = "0"
	}
	err = json.NewEncoder(w).Encode(&result)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetLastSeenID(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)

	maxid, err := req.Db.GetLastSeenID(ctx)
	result := map[string]string{}
	result["status"] = "ok"
	result["msg"] = "new notes count"
	result["data"] = fmt.Sprintf("%d", maxid)

	if err != nil {
		result["status"] = "error"
		result["msg"] = err.Error()
		result["data"] = "0"
	}
	err = json.NewEncoder(w).Encode(&result)
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
	ev, _ := req.Db.FindEvent(ctx, j.ID)
	if ev.Event.ID == "" {
		filter := nostr.Filter{
			IDs:   []string{j.ID},
			Limit: 1,
		}

		evs := req.Nostr.GetEvents(filter)

		req.Db.SaveEvents(ctx, evs)

		log.Println("Need to get it", j.ID, filter)
	}
	ev, _ = req.Db.FindEvent(ctx, j.ID)

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
	var mu sync.Mutex

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
	ctx := context.Background()

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
	var postEv nostr.Event
	if msg.Event_id == "" {
		postEv, _ = req.Nostr.DoPost(msg.Msg)

		req.Db.SaveEvents(ctx, []*nostr.Event{&postEv})
	}

	result := map[string]string{}
	if msg.Event_id != "" {
		replyEv, _ := req.Db.FindRawEvent(ctx, msg.Event_id)
		postEv, _ = req.Nostr.DoReply(msg.Msg, *replyEv)

		req.Db.SaveEvents(ctx, []*nostr.Event{&postEv})
	}

	result["status"] = "ok"
	result["msg"] = msg.Msg
	result["reply_to_event_id"] = msg.Event_id
	jsonPostEv, _ := json.Marshal(postEv)
	result["post"] = string(jsonPostEv)

	if err != nil {
		log.Println(err)
		result["status"] = "error"
		result["msg"] = err.Error()
	}

	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		panic(err)
	}
}

func (req *Requests) GetMetaData(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	event, _ := req.Nostr.GetMetaData()

	req.Db.SaveProfiles(ctx, []*nostr.Event{&event})

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

	var user nostrWrapper.Profile
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		log.Println(err)
		panic(err)
	}
	user.Pubkey = req.Cfg.PubKey
	_ = req.Nostr.DoPublishMetaData(&user)

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
	ctx := context.Background()

	profile, _ := req.Db.FindProfile(ctx, req.Cfg.PubKey)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(profile)
	if err != nil {
		panic(err)
	}
}
