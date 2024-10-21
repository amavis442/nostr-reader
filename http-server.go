package main

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
)

type ServerConfig struct {
	Port     int64
	Frontend string
	Interval int64
}

type HttpServer struct {
	DevMode  bool
	Server   *ServerConfig
	Database *Storage
	Nostr    *Wrapper
}

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

func (httpServer *HttpServer) Start() {
	var req RequestHandler
	req.Pubkey = httpServer.Nostr.Cfg.PubKey
	req.Db = httpServer.Database
	req.Nostr = httpServer.Nostr

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

	if !(httpServer.Server.Interval > 0) {
		log.Println("Please set the interval in minutes in config.json")
		os.Exit(0)
	}
	var port string = "8080"
	if httpServer.Server.Port > 0 {
		port = fmt.Sprint(httpServer.Server.Port)
	}

	fmt.Println("Server running: http://localhost:" + port)

	err := http.ListenAndServe(":"+port, mux)
	if err != nil {
		log.Println("Could not start http server on this port: " + port)
		log.Fatal(err)
	}
}
