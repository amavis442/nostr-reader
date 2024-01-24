package main

import (
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

	var ctx context.Context = context.Background()
	var st Storage
	err = st.Connect(ctx, cfg) // Does not make a connection immediatly but prepares so it does not yet know if the pg server is available.
	if err != nil {
		log.Println(err.Error())
		os.Exit(0)
	}
	// close database
	defer st.Close()

	err = st.CreateTables(ctx) // Here it knows if the pg database is available.
	if err != nil {
		log.Println(err.Error())
		os.Exit(0)
	}

	st.Filter = cfg.Filter
	cfg.Storage = &st

	var nostr Nostr
	nostr.Cfg = cfg
	nostr.Storage = &st

	var req Requests
	req.Cfg = cfg
	req.Nostr = &nostr

	// Windows may be missing this
	mime.AddExtensionType(".js", "application/javascript")

	mux := http.NewServeMux()
	/*
	 * Get events that already are stored in the database
	 * This will not SYNC the local database with that of the relays.
	 */
	mux.HandleFunc("/api/events", req.getRoot)
	mux.HandleFunc("/api/inbox", req.getInbox)
	/**
	 * This will sync the local database with that of the relays (Only public events and not channels and such)
	 */
	mux.HandleFunc("/api/sync", req.StartSync)
	mux.HandleFunc("/api/syncnote", req.SyncNote)

	/**
	 * Put a user on the naughty list
	 */
	mux.HandleFunc("/api/blockuser", req.BlockUser)

	/**
	 * Put a user on the follow list
	 * This is all local and will not send an event for followlist
	 */
	mux.HandleFunc("/api/followuser", req.FollowUser)
	mux.HandleFunc("/api/unfollowuser", req.UnfollowUser)
	mux.HandleFunc("/api/getfollownotes", req.FollowUserNotes)

	/**
	 * Bookmark events you want to keep track of
	 */
	mux.HandleFunc("/api/bookmark", req.BookMark)
	mux.HandleFunc("/api/removebookmark", req.RemoveBookMark)
	mux.HandleFunc("/api/getbookmarked", req.GetBookMarked)

	/**
	 * Find an event based on event id. This can be a reply
	 */
	mux.HandleFunc("/api/searchevent", req.SearchEvent)

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

	mux.Handle("/", http.FileServer(http.Dir("web/nostr-reader/dist")))

	var port string = "8080"
	if cfg.Server.Port > 0 {
		port = fmt.Sprint(cfg.Server.Port)
	}

	srv := &http.Server{
		Addr:           ":" + port,
		Handler:        mux,
		ReadTimeout:    45 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	ticker := time.NewTicker(300 * time.Second)
	// Creating channel using make
	tickerChan := make(chan bool)

	go func() {
		for {
			select {
			case <-tickerChan:
				return
			// interval task
			case tm := <-ticker.C:
				fmt.Println("The Current time is: ", tm)
				intervalTask(nostr)
			}
		}
	}()

	fmt.Println("Server running: http://localhost:" + port)
	//log.Fatal(http.ListenAndServe(":"+port, mux))
	log.Fatal(srv.ListenAndServe())

	// Turn off ticker after 10 seconds
	// Calling Sleep() method
	//time.Sleep(10 * time.Second)

	// Calling Stop() method
	//ticker.Stop()

	// Setting the value of channel
	//tickerChan <- true

	// Printed when the ticker is turned off
	//fmt.Println("Ticker is turned off!")
}

func intervalTask(nostr Nostr) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	EventsQueue = EventsQueue[:0]
	nostr.getEventData(ctx)
}
