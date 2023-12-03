package main

import (
	"log"
	"mime"
	"net/http"
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
		panic(err)
	}

	var st Storage
	st.Connect(cfg)
	st.CreateTables()

	st.Filter = cfg.Filter
	cfg.Storage = &st

	var nostr Nostr
	nostr.Cfg = cfg
	nostr.Storage = &st

	var req Requests
	req.Cfg = cfg
	req.Nostr = &nostr

	// close database
	defer st.Close()

	// Windows may be missing this
	mime.AddExtensionType(".js", "application/javascript")

	/*
	 * Get events that already are stored in the database
	 * This will not SYNC the local database with that of the relays.
	 */
	http.HandleFunc("/api/events", req.getRoot)

	/**
	 * This will sync the local database with that of the relays (Only public events and not channels and such)
	 */
	http.HandleFunc("/api/sync", req.StartSync)

	/**
	 * Put a user on the naughty list
	 */
	http.HandleFunc("/api/blockuser", req.BlockUser)

	/**
	 * Put a user on the follow list
	 * This is all local and will not send an event for followlist
	 */
	http.HandleFunc("/api/followuser", req.FollowUser)

	/**
	 * Find an event based on event id. This can be a reply
	 */
	http.HandleFunc("/api/searchevent", req.SearchEvent)

	/**
	 * Sometimes it is nice to see pictures in the post and not just a link
	 */
	http.HandleFunc("/api/preview/link", req.PreviewLink)

	/**
	 * Sometimes it is nice to see pictures in the post and not just a link
	 */
	http.HandleFunc("/api/publish", req.Publish)

	http.Handle("/", http.FileServer(http.Dir("web/nostr-reader/dist")))

	log.Fatal(http.ListenAndServe(":8080", nil))

}
