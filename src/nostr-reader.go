package main

import (
	"amavis442/nostr-reader/database"
	"amavis442/nostr-reader/nostr/wrapper"
	"context"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"time"
)

/**
 * Main app
 * This file is used from processing http requests from the frontend
 */

/**
 * Process all the http calls
 */
func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Println(err.Error())
		os.Exit(0)
	}
	if cfg.Server.Interval < 1 {
		log.Println("Setting interval to 1 minute. This is the minimum")
		cfg.Server.Interval = 1
	}

	var ctx context.Context = context.Background()
	var st database.Storage
	var nostrWrapper wrapper.NostrWrapper
	nostrWrapper.SetConfig(&cfg.Config)
	st.SetEnvironment(cfg.Env)

	err = st.Connect(ctx, cfg.Database) // Does not make a connection immediatly but prepares so it does not yet know if the pg server is available.
	if err != nil {
		log.Println(err.Error())
		os.Exit(0)
	}

	relays := st.GetRelays(ctx)
	UpdateRelays(&nostrWrapper.Cfg, relays)

	var req Requests
	req.Cfg = cfg
	req.Db = &st
	req.Nostr = &nostrWrapper

	// Windows may be missing this
	mime.AddExtensionType(".js", "application/javascript")

	mux := http.NewServeMux()
	/*
	 * Get events that already are stored in the database
	 * This will not SYNC the local database with that of the relays.
	 */
	mux.HandleFunc("/api/getnotes", req.GetNotes)
	mux.HandleFunc("/api/getinbox", req.GetInbox)

	mux.HandleFunc("/api/getnewnotescount", req.GetNewNotesCount)

	mux.HandleFunc("/api/getlastseenid", req.GetLastSeenID)

	/**
	 * Put a user on the naughty list
	 */
	mux.HandleFunc("/api/blockuser", req.BlockUser)

	/**
	 * Put a user on the follow list
	 * This is all local and will not send an event for followlist
	 */
	mux.HandleFunc("/api/followuser", req.Follow)
	mux.HandleFunc("/api/unfollowuser", req.Unfollow)
	mux.HandleFunc("/api/getfollownotes", req.GetFollowedNotes)
	mux.HandleFunc("/api/getfollowed", req.GetFollowedProfiles)

	/**
	 * Bookmark events you want to keep track of
	 */
	mux.HandleFunc("/api/bookmark", req.AddBookMark)
	mux.HandleFunc("/api/removebookmark", req.RemoveBookMark)
	mux.HandleFunc("/api/getbookmarked", req.GetBookMarked)

	/**
	* Relay settings
	 */
	mux.HandleFunc("/api/addrelay", req.AddRelay)
	mux.HandleFunc("/api/removerelay", req.RemoveRelay)
	mux.HandleFunc("/api/getrelays", req.GetRelays)

	/**
	 * Sometimes it is nice to see pictures in the post and not just a link
	 */
	mux.HandleFunc("/api/preview/link", req.PreviewLink)

	/**
	 * Sometimes it is nice to see pictures in the post and not just a link
	 */
	mux.HandleFunc("/api/publish", req.Publish)

	/**
	 * Use meta data set and get
	 */
	mux.HandleFunc("/api/getmetadata", req.GetMetaData)
	mux.HandleFunc("/api/setmetadata", req.SetMetaData)
	mux.HandleFunc("/api/getprofile", req.GetProfile)

	mux.Handle("/", http.FileServer(http.Dir(cfg.Server.Frontend)))

	var port string = "8080"
	if cfg.Server.Port > 0 {
		port = fmt.Sprint(cfg.Server.Port)
	}

	if !(cfg.Server.Interval > 0) {
		log.Println("Please set the interval in minutes in config.json")
		os.Exit(0)
	}
	intervalTimer := time.Duration(cfg.Server.Interval * 60)
	ticker := time.NewTicker(intervalTimer * time.Second)

	// Creating channel using make
	tickerChan := make(chan bool)
	go func() {
		for {
			select {
			case <-tickerChan:
				return
			// interval task
			case tm := <-ticker.C:
				log.Println("The Current time is: ", tm)
				go intervalTask(ctx, &req, 20)
			}
		}
	}()

	intervalTask(ctx, &req, 60)
	fmt.Println("Server running: http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func intervalTask(ctx context.Context, req *Requests, timeOut int) {
	tOut := time.Duration(timeOut) * time.Second
	ctx, cancel := context.WithTimeout(ctx, tOut)
	defer cancel()

	createdAt := req.Db.GetLastTimeStamp(ctx)
	t := time.Unix(createdAt, 0)
	log.Println("TimeStamps: ", createdAt, t.UTC())
	filter := req.Nostr.GetEventData(createdAt, false)
	evs := req.Nostr.GetEvents(ctx, filter)

	_, err := req.Db.SaveEvents(ctx, evs)
	if err != nil {
		log.Fatal(err)
	}
}
