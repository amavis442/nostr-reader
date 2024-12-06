package http

import (
	"amavis442/nostr-reader/internal/db"
	"amavis442/nostr-reader/internal/logger"
	wrapper "amavis442/nostr-reader/internal/nostr"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/render"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

type Controller struct {
	Pubkey string
	Db     *db.Storage
	Nostr  *wrapper.Wrapper
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

type Msg struct {
	Msg      string `json:"msg"`
	Event_id string `json:"event_id"` // If it is a reply
}

type Url struct {
	Url string `json:"url"`
}

/**
 * Not all events are processed at once and we do not want to miss out on events, so put them in a queque and use FIFO to process.
 */
//var EventsQueue = make([]nostr.Event, 0)

// var ptagsQueue = make([]string, 0)
var syncHash string = ""

// Send from client API
type PageRequest struct {
	Cursor     uint64 `json:"cursor"`
	PrevCursor uint64 `json:"prev_cursor"`
	NextCursor uint64 `json:"next_cursor"`
	PerPage    uint   `json:"per_page"`
	Since      uint   `json:"since"`
	Renew      bool   `json:"renew"`
	Context    string `json:"context"`
}

// Response godoc
// @Description  Standard response to return to client
type Response struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	Context string      `json:"context"`
}

// ResponseEventData godoc
// @Description  Paginated response to return to client
type ResponseEventData struct {
	Paging *db.Pagination `json:"paging"`
	Events *[]db.Event    `json:"events"`
}

type ResponseProfile struct {
	Result map[string]string `json:"result"`
}

type ResponseRelay struct {
	Response
	Relays []db.Relay `json:"relays"`
}

func (c *Controller) parseUrlParams(r *http.Request) PageRequest {
	var err error
	var p PageRequest
	cursor := r.URL.Query().Get("cursor")
	p.Cursor, _ = strconv.ParseUint(cursor, 10, 64)

	next_cursor := r.URL.Query().Get("next_cursor")
	p.NextCursor, _ = strconv.ParseUint(next_cursor, 10, 64)

	prev_cursor := r.URL.Query().Get("prev_cursor")
	p.PrevCursor, _ = strconv.ParseUint(prev_cursor, 10, 64)

	per_page := r.URL.Query().Get("per_page")
	perpage, _ := strconv.ParseUint(per_page, 10, 64)
	p.PerPage = uint(perpage)

	renew := r.URL.Query().Get("renew")
	p.Renew, err = strconv.ParseBool(renew)
	if err != nil {
		p.Renew = false
	}

	p.Context = r.URL.Query().Get("context")

	since := r.URL.Query().Get("since")
	sinceI, _ := strconv.ParseUint(since, 10, 64)
	p.Since = uint(sinceI)

	return p
}

/**
* The API requests
 */
// GetNotes godoc
// @Summary      Retrieve stored Notes
// @Description  get Notes
// @Tags         notes
// @Accept       json
// @Produce      json
// @Param		 cursor	query	int	true	"Cursor"
// @Param		 start_id	query	int	true	"Start id"
// @Param		 end_id		query	int	true	"End id"
// @Param		 per_page	query	int	false	"Results per page"	Default(10)
// @Param		 renew		query	bool	false	"Renew page and ignore start_id" Default(false)
// @Param		 since		query	int	false	"Since"
// @Param		context	query string false "string enum" Enums(follow, bookmark, refresh, global)
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/getnotes [get]
func (c *Controller) GetNotes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		p := c.parseUrlParams(r)

		pagination := db.Pagination{}
		pagination.SetPerPage(uint(p.PerPage))
		pagination.SetCursor(p.Cursor)
		pagination.SetPrev(p.PrevCursor)
		pagination.SetNext(p.NextCursor)
		pagination.SetPerPage(p.PerPage)
		pagination.SetSince(p.Since)

		var options db.Options
		switch p.Context {
		case "refresh":
			options = db.Options{Follow: true, BookMark: false, Renew: p.Renew}
		case "refresh.global":
			options = db.Options{Follow: false, BookMark: false, Renew: p.Renew}
		case "follow":
			options = db.Options{Follow: true, BookMark: false, Renew: p.Renew}
		case "bookmark":
			options = db.Options{Follow: false, BookMark: true, Renew: p.Renew}
		case "global":
			options = db.Options{Follow: false, BookMark: false, Renew: p.Renew}
		default:
			options = db.Options{Follow: true, BookMark: false, Renew: p.Renew}
		}

		events, err := c.Db.GetNotes(ctx, p.Context, &pagination, options)
		if err != nil {
			slog.Error(logger.GetCallerInfo(1), "error", err.Error())
		}

		data := &ResponseEventData{Paging: &pagination, Events: events}
		response := &Response{}

		response.Status = "ok"
		response.Message = "Data ok"
		if len(*data.Events) == 0 {
			response.Status = "empty"
			response.Message = "No results"
			pagination.SetPerPage(uint(p.PerPage))
			pagination.SetCursor(p.Cursor)
			pagination.SetPrev(p.PrevCursor)
			pagination.SetNext(0)
			pagination.SetPerPage(p.PerPage)
			pagination.SetSince(p.Since)
			data.Paging = &pagination
			data.Events = events
		}

		if err != nil {
			response.Status = "failed"
			response.Message = err.Error()
		}
		response.Data = data
		render.JSON(w, r, response)
	}
}

// GetInbox godoc
// @Summary      Retrieve stored Notes
// @Description  get Notes that you responded to
// @Tags         notes
// @Accept       json
// @Produce      json
// @Param        Body body PageRequest true "Body for the retrieval of data"
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/getinbox [get]
func (c *Controller) GetInbox() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		p := c.parseUrlParams(r)

		pagination := db.Pagination{}
		pagination.SetPerPage(uint(p.PerPage))
		pagination.SetCursor(p.Cursor)
		pagination.SetPrev(p.PrevCursor)
		pagination.SetNext(p.NextCursor)
		pagination.SetPerPage(p.PerPage)
		pagination.SetSince(p.Since)

		events, err := c.Db.GetInbox(ctx, p.Context, &pagination, c.Pubkey)

		data := &ResponseEventData{Paging: &pagination, Events: events}
		response := &Response{}
		response.Status = "ok"
		response.Message = "Pagination results"
		if err != nil {
			response.Status = "failed"
			response.Message = err.Error()
		}
		response.Data = data
		render.JSON(w, r, response)
	}
}

func (c *Controller) GetNotifications() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		p := c.parseUrlParams(r)

		pagination := db.Pagination{}
		pagination.SetPerPage(uint(p.PerPage))
		pagination.SetCursor(p.Cursor)
		pagination.SetPrev(p.PrevCursor)
		pagination.SetNext(p.NextCursor)
		pagination.SetPerPage(p.PerPage)
		pagination.SetSince(p.Since)

		events, err := c.Db.GetNotifications(ctx, &pagination)

		data := &ResponseEventData{Paging: &pagination, Events: events}
		response := &Response{}
		response.Status = "ok"
		response.Message = "Pagination results"
		if err != nil {
			response.Status = "failed"
			response.Message = err.Error()
		}
		response.Data = data
		render.JSON(w, r, response)
	}
}

func (c *Controller) StartSync() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		createdAt := c.Db.GetLastTimeStamp(ctx)

		filter := c.Nostr.GetEventData(createdAt, true)
		evs := c.Nostr.GetEvents(ctx, filter)
		var pubkeys = make([]string, 0)
		var err error
		pubkeys, err = c.Db.SaveEvents(ctx, evs)
		if err != nil {
			response.Status = "error"
			response.Message = err.Error()
		}

		// Todo build check for ttl so user data is not refreshed every time.
		var tresholdTime int64 = time.Now().Unix() - 60*60*24

		pubkeys, _ = c.Db.CheckProfiles(ctx, pubkeys, tresholdTime)
		// Last but not least, try to get the user metadata
		c.Nostr.UpdateProfiles(ctx, pubkeys)
		err = c.Db.SaveProfiles(ctx, evs)
		if err != nil {
			response.Status = "error"
			response.Message = err.Error()
		}

		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			panic(err)
		}
	}
}

func (c *Controller) SyncNote() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		evs := c.Nostr.GetEvents(ctx, filter)

		c.Db.SaveEvents(ctx, evs)
		ev, _ := c.Db.FindEvent(ctx, j.ID)

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
}

// BlockUser godoc
// @Summary      Block an anoying user
// @Description  Block user
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        Body body Pubkey true "Body for the retrieval of data"
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/blockuser [post]
func (c *Controller) BlockUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		c.Db.CreateBlock(ctx, user.Pubkey)

		response := &Response{}
		response.Status = "ok"
		response.Message = "Blocked pubkey: " + user.Pubkey
		response.Data = user.Pubkey

		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			panic(err)
		}
	}
}

// Follow godoc
// @Summary      Follow a user
// @Description  Follow a user
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        Body body Pubkey true "Body for the retrieval of data"
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/followuser [post]
func (c *Controller) Follow() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		err = c.Db.CreateFollow(ctx, user.Pubkey)
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
}

// Unfollow godoc
// @Summary      Unfollow a user
// @Description  UnFollow a user
// @Tags         user
// @Accept       json
// @Produce      json
// @Param        Body body Pubkey true "Body for the retrieval of data"
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/unfollowuser [post]
func (c *Controller) Unfollow() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		err = c.Db.RemoveFollow(ctx, user.Pubkey)

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
}

// GetFollowedProfiles godoc
// @Summary      Profiles of the followed users
// @Description  Profiles of the followed users
// @Tags         user
// @Accept       json
// @Produce      json
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/getfollowed [get]
func (c *Controller) GetFollowedProfiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		profiles := c.Db.GetFollowedProfiles(ctx)

		response := &Response{}
		response.Data = profiles
		response.Status = "ok"
		response.Message = "Profiles"

		err := json.NewEncoder(w).Encode(&response)
		if err != nil {
			panic(err)
		}
	}
}

// AddBookMark godoc
// @Summary      Bookmark a note
// @Description  Bookmark a note
// @Tags         bookmark
// @Accept       json
// @Produce      json
// @Param        Body body BookMark true "Body for the retrieval of data"
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/bookmark [post]
func (c *Controller) AddBookMark() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		err = c.Db.CreateBookMark(ctx, j.EventId)

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
}

// RemoveBookMark godoc
// @Summary      Remove bookmark from note
// @Description  Remove bookmark from note
// @Tags         bookmark
// @Accept       json
// @Produce      json
// @Param        Body body BookMark true "Body for the retrieval of data"
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/removebookmark [post]
func (c *Controller) RemoveBookMark() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		err = c.Db.RemoveBookMark(ctx, j.EventId)

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
}

// AddRelay godoc
// @Summary      Add relay
// @Description  Add relay
// @Tags         relay
// @Accept       json
// @Produce      json
// @Param        Body body Relay true "Body for the retrieval of data"
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/addrelay [post]
func (c *Controller) AddRelay() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		var j db.Relay
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

		err = c.Db.CreateRelay(ctx, &j)
		if err != nil {
			response.Status = "error"
			response.Message = err.Error()
		}

		relays := c.Db.GetRelays(ctx)
		response.Data = relays

		c.Nostr.UpdateRelays(relays)

		err = json.NewEncoder(w).Encode(&response)
		if err != nil {
			panic(err)
		}
	}
}

// RemoveRelay godoc
// @Summary      Remove relay
// @Description  Remove relay
// @Tags         relay
// @Accept       json
// @Produce      json
// @Param        Body body Relay true "Body for the retrieval of data"
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/removerelay [post]
func (c *Controller) RemoveRelay() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		var j db.Relay
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
		err = c.Db.RemoveRelay(ctx, j.Url)
		if err != nil {
			response.Status = "error"
			response.Message = err.Error()
		}

		relays := c.Db.GetRelays(ctx)
		response.Data = relays

		c.Nostr.UpdateRelays(relays)

		err = json.NewEncoder(w).Encode(&response)
		if err != nil {
			panic(err)
		}
	}
}

// GetRelays godoc
// @Summary      Get relays
// @Description  Get relays
// @Tags         relay
// @Accept       json
// @Produce      json
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/getrelays [get]
func (c *Controller) GetRelays() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)

		response := &Response{}
		response.Status = "ok"
		response.Message = "Relays"
		relays := c.Db.GetRelays(ctx)
		response.Data = relays

		err := json.NewEncoder(w).Encode(&response)
		if err != nil {
			panic(err)
		}
	}
}

// GetNewNotesCount godoc
// @Summary      Get count of new notes
// @Description  Get count of new notes
// @Tags         notes
// @Accept       json
// @Produce      json
// @Param		 cursor	query	int	true	"Cursor"
// @Param		 start_id	query	int	true	"Start id"
// @Param		 end_id		query	int	true	"End id"
// @Param		 per_page	query	int	false	"Results per page"	Default(10)
// @Param		 renew		query	bool	false	"Renew page and ignore start_id" Default(false)
// @Param		 since		query	int	false	"Since"
// @Param		context	query string false "string enum" Enums(follow, bookmark, refresh, global)
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/getnewnotescount [get]
func (c *Controller) GetNewNotesCount() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		var err error
		var p PageRequest
		cursor := r.URL.Query().Get("cursor")
		p.Cursor, _ = strconv.ParseUint(cursor, 10, 64)

		renew := r.URL.Query().Get("renew")
		p.Renew, _ = strconv.ParseBool(renew)

		p.Context = r.URL.Query().Get("context")

		var options db.Options
		switch p.Context {
		case "refresh":
			options = db.Options{Follow: true, BookMark: false, Renew: p.Renew}
		case "follow":
			options = db.Options{Follow: true, BookMark: false, Renew: p.Renew}
		case "bookmark":
			options = db.Options{Follow: false, BookMark: true, Renew: p.Renew}
		case "global":
			options = db.Options{Follow: false, BookMark: false, Renew: p.Renew}
		default:
			options = db.Options{Follow: true, BookMark: false, Renew: p.Renew}
		}

		response := &Response{}
		response.Status = "ok"
		response.Message = "new notes count"
		response.Context = p.Context

		count, err := c.Db.GetNewNotesCount(ctx, p.Cursor, options)

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
}

// GetLastSeenID godoc
// @Summary      Last seen note id
// @Description  Last seen note id
// @Tags         notes
// @Accept       json
// @Produce      json
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/getlastseenid [get]
func (c *Controller) GetLastSeenID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*") // for CORS
		w.WriteHeader(http.StatusOK)

		maxid, err := c.Db.GetLastSeenID(ctx)

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
}

func (c *Controller) SearchEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		ev, _ := c.Db.FindEvent(ctx, j.ID)
		if ev.Event.ID == "" {
			filter := nostr.Filter{
				IDs:   []string{j.ID},
				Limit: 1,
			}

			evs := c.Nostr.GetEvents(ctx, filter)

			c.Db.SaveEvents(ctx, evs)

			log.Println("Need to get it", j.ID, filter)
		}
		ev, _ = c.Db.FindEvent(ctx, j.ID)

		response := &Response{}
		response.Status = "ok"
		response.Message = "Result find for event: " + j.ID
		response.Data = ev

		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			panic(err)
		}
	}
}

/**
 * Need an easy way to cancel this request when a new nextpage or refreshpage comes in
 */
func (c *Controller) PreviewLink() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var mu sync.Mutex

		defer r.Body.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		var url Url
		err := json.NewDecoder(r.Body).Decode(&url)
		if err != nil {
			panic(err)
		}

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
}

// Publish godoc
// @Summary      Get count of new notes
// @Description  Get count of new notes
// @Tags         publish
// @Accept       json
// @Produce      json
// @Param        Body body Msg true "Body for the retrieval of data"
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/publish [post]
func (c *Controller) Publish() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		var msg Msg
		err := json.NewDecoder(r.Body).Decode(&msg)
		if err != nil {
			panic(err)
		}

		w.WriteHeader(http.StatusOK)

		log.Println("Msg to publish: ", msg.Msg)
		var postEv db.Event

		if msg.Event_id == "" {
			postEv, err = c.Nostr.DoPost(msg.Msg)
			if err != nil {
				slog.Warn("something went wrong creating post for broadcasting", "error", err.Error())
			}
		}

		if msg.Event_id != "" {
			replyEv, err := c.Db.FindRawEvent(ctx, msg.Event_id)
			if err != nil {
				slog.Warn(logger.GetCallerInfo(1)+" Something went wrong", "error", err.Error())
			}
			postEv, err = c.Nostr.DoReply(msg.Msg, *replyEv)
			if err != nil {
				slog.Warn("Something went wrong creating post for broadcasting: ", "error", err.Error())
			}

		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func(wg *sync.WaitGroup, c *Controller, postEv *db.Event) {
			c.Db.SaveNote(ctx, postEv)
			if err != nil {
				slog.Warn(logger.GetCallerInfo(1), "error", err.Error())
			}
			wg.Done()
		}(&wg, c, &postEv)

		wg.Add(1)

		go func(ctx context.Context, c *Controller, postEv *db.Event) {
			success, err := c.Nostr.BroadCast(ctx, *postEv)
			if err != nil {
				slog.Warn(logger.GetCallerInfo(1)+"cannot broadcast", "error", err.Error())
			}
			if !success {
				slog.Warn(logger.GetCallerInfo(1) + "cannot broadcast succesfully")
			}
			wg.Done()
		}(ctx, c, &postEv)

		response := &Response{}
		response.Status = "ok"
		response.Message = msg.Msg

		jsonPostEv, err := json.Marshal(postEv)
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

		wg.Wait()
	}

}

// SetMetaData godoc
// @Summary      Set your profile data
// @Description  Set your profile data
// @Tags         profile
// @Accept       json
// @Produce      json
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/getmetadata [get]
func (c *Controller) GetMetaData() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		response := &Response{}
		event, err := c.Nostr.GetMetaData(ctx)
		response.Data = event
		if err != nil {
			response.Status = "error"
			response.Message = err.Error()
			render.JSON(w, r, response)
			return
		}

		err = c.Db.SaveProfiles(ctx, []*db.Event{&event})
		if err != nil {
			response.Status = "error"
			response.Message = err.Error()
			render.JSON(w, r, response)
			return
		}

		data := &event
		response.Status = "ok"
		response.Message = "Metadata"
		response.Data = data
		render.JSON(w, r, response)
	}
}

// SetMetaData godoc
// @Summary      Publish your profile data
// @Description  Publish your profiledata to relays
// @Tags         profile
// @Accept       json
// @Produce      json
// @Param        Body body Profile true "Body for the retrieval of data"
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/setmetadata [post]
func (c *Controller) SetMetaData() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var user db.Profile
		err := json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			log.Println(err)
			panic(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		user.Pubkey = c.Pubkey
		err = c.Nostr.DoPublishMetaData(ctx, &user)

		response := &Response{}
		response.Status = "ok"
		response.Message = "Set metadata"
		response.Data = &user

		if err != nil {
			response.Status = "error"
			response.Message = err.Error()
		}

		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			panic(err)
		}
	}
}

// GetProfile godoc
// @Summary      Get your profile data
// @Description  Get your profile data
// @Tags         profile
// @Accept       json
// @Produce      json
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/getprofile [get]
func (c *Controller) GetProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()

		profile, _ := c.Db.FindProfile(ctx, c.Pubkey)

		response := &Response{}
		response.Status = "ok"
		response.Message = "Profile"
		response.Data = &profile

		jsonStr, err := json.Marshal(profile)
		slog.Info(string(jsonStr))
		if err != nil {
			response.Status = "error"
			response.Message = err.Error()
		}
		render.JSON(w, r, response)
	}
}

// SearchProfiles godoc
// @Summary      Search for profiles
// @Description  Search for profiles
// @Tags         profile
// @Accept       json
// @Produce      json
// @Success      200  {object}  Response
// @Failure      400  {string}  string    "error"
// @Failure      404  {string}  string    "error"
// @Failure      500  {string}  string    "error"
// @Router       /api/searchprofiles [get]
func (c *Controller) SearchProfiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		response := &Response{}
		response.Status = "ok"
		response.Message = "Profile"

		//var err error
		searchStr := r.URL.Query().Get("q")
		var pubkey string
		pubkey = searchStr
		if searchStr[0:4] == "npub" {
			prefix, val, err := nip19.Decode(searchStr)
			if err != nil {
				response.Status = "error"
				response.Message = err.Error()
			}
			if err == nil && prefix == "npub" {
				pubkey = val.(string)
			}
		}

		profiles, err := c.Db.SearchProfiles(ctx, pubkey)
		if len(*profiles) > 0 {
			response.Data = &profiles
		}
		if err != nil {
			response.Status = "error"
			response.Message = err.Error()
		}

		if len(*profiles) < 1 && err == nil {
			event, err := c.Nostr.SearchProfileByNpub(ctx, pubkey)
			if err != nil {
				slog.Error(err.Error())
				response.Status = "error"
				response.Message = err.Error()
			}
			if err == nil {
				c.Db.SaveProfile(ctx, &event)
				profile, _ := c.Db.FindProfile(ctx, event.Event.PubKey)
				profiles := make([]db.Profile, 0)
				profiles = append(profiles, profile)
				response.Data = profiles
			}
		}

		render.JSON(w, r, response)
	}
}
