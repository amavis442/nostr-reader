package main

import (
	"amavis442/nostr-reader/database"
	"amavis442/nostr-reader/nostr/wrapper"
	"context"
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"sync"
	"time"
)

func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		w.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")

		if r.Method == "OPTIONS" {
			http.Error(w, "No Content", http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

/**
 * Main app
 * This file is used from processing http requests from the frontend
 */

/**
 * Process all the http calls
 */
func main() {
	devMode := false

	modePtr := flag.String("mode", "prod", "Production or development? Valid options are prod or dev")
	helpPtr := flag.Bool("h", false, "Show help dialog")

	flag.Parse()

	if *modePtr == "dev" {
		devMode = true
	}
	if *modePtr != "dev" && *modePtr != "prod" {
		fmt.Println("Unkown mode: ", *modePtr, ". See -mode for valid modes")
		return
	}
	if *helpPtr {
		fmt.Println("Use -mode=dev when working on frontend. It still does the same thing as in production")
		return
	}

	fmt.Println("Mode is: ", *modePtr)

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
	st.Pubkey = cfg.PubKey

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
	mux.HandleFunc("/api/getnotes", CORS(req.GetNotes))
	mux.HandleFunc("/api/getinbox", CORS(req.GetInbox))

	mux.HandleFunc("/api/getnewnotescount", CORS(req.GetNewNotesCount))

	mux.HandleFunc("/api/getlastseenid", CORS(req.GetLastSeenID))

	/**
	 * Put a user on the naughty list
	 */
	mux.HandleFunc("/api/blockuser", CORS(req.BlockUser))

	/**
	 * Put a user on the follow list
	 * This is all local and will not send an event for followlist
	 */
	mux.HandleFunc("/api/followuser", CORS(req.Follow))
	mux.HandleFunc("/api/unfollowuser", CORS(req.Unfollow))
	mux.HandleFunc("/api/getfollownotes", CORS(req.GetFollowedNotes))
	mux.HandleFunc("/api/getfollowed", CORS(req.GetFollowedProfiles))

	/**
	 * Bookmark events you want to keep track of
	 */
	mux.HandleFunc("/api/bookmark", CORS(req.AddBookMark))
	mux.HandleFunc("/api/removebookmark", CORS(req.RemoveBookMark))
	mux.HandleFunc("/api/getbookmarked", CORS(req.GetBookMarked))

	/**
	* Relay settings
	 */
	mux.HandleFunc("/api/addrelay", CORS(req.AddRelay))
	mux.HandleFunc("/api/removerelay", CORS(req.RemoveRelay))
	mux.HandleFunc("/api/getrelays", CORS(req.GetRelays))

	/**
	 * Sometimes it is nice to see pictures in the post and not just a link
	 */
	mux.HandleFunc("/api/preview/link", CORS(req.PreviewLink))

	/**
	 * Sometimes it is nice to see pictures in the post and not just a link
	 */
	mux.HandleFunc("/api/publish", CORS(req.Publish))

	/**
	 * Use meta data set and get
	 */
	mux.HandleFunc("/api/getmetadata", CORS(req.GetMetaData))
	mux.HandleFunc("/api/setmetadata", CORS(req.SetMetaData))
	mux.HandleFunc("/api/getprofile", CORS(req.GetProfile))

	if cfg.Env == "prod" {
		mux.Handle("/", http.FileServer(http.Dir(cfg.Server.Frontend)))
	}

	if !(cfg.Server.Interval > 0) {
		log.Println("Please set the interval in minutes in config.json")
		os.Exit(0)
	}
	intervalTimer := time.Duration(cfg.Server.Interval * 60)
	ticker := time.NewTicker(intervalTimer * time.Second)

	var wg sync.WaitGroup
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
				wg.Add(1)
				go intervalTask(&wg, ctx, &req, 120)
			}
		}
	}()

	var port string = "8080"
	if cfg.Server.Port > 0 {
		port = fmt.Sprint(cfg.Server.Port)
	}

	fmt.Println("Server running: http://localhost:" + port)
	if devMode {
		fmt.Println("Running in dev mode, so no frontend.")
	}
	err = http.ListenAndServe(":"+port, mux)
	if err != nil {
		log.Println("Could not start http server on this port: " + port)
		log.Fatal(err)
	}

	wg.Wait()
}

func intervalTask(wg *sync.WaitGroup, ctx context.Context, req *Requests, timeOut int) {
	tOut := time.Duration(timeOut) * time.Second
	ctx, cancel := context.WithTimeout(ctx, tOut)
	defer func() {
		wg.Done()
		cancel()
	}()

	createdAt := req.Db.GetLastTimeStamp(ctx)
	t := time.Unix(createdAt, 0)
	log.Println("TimeStamps: ", createdAt, t.UTC())
	filter := req.Nostr.GetEventData(createdAt, false)
	evs := req.Nostr.GetEvents(ctx, filter)

	_, err := req.Db.SaveEvents(ctx, evs)
	if err != nil {
		log.Println(err)
	}
	log.Println("Done syncing")

}
