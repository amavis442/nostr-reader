package main

import (
	"context"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
)

/**
 * Main app
 * This file is used from processing http requests from the frontend
 */

/**
 * Process all the http calls
 */
func main() {
	f, err := os.OpenFile("relaystore.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	log.SetOutput(f)

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

	/**
	 * This will sync the local database with that of the relays (Only public events and not channels and such)
	 */
	mux.HandleFunc("/api/sync", req.StartSync)

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

	mux.Handle("/", http.FileServer(http.Dir("web/nostr-reader/dist")))

	fmt.Println("Server running: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))

}
