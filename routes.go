package main

import (
	_ "amavis442/nostr-reader/docs"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	httpSwagger "github.com/swaggo/http-swagger"
)

func routes(c *Controller, port string) *chi.Mux {
	router := chi.NewRouter()

	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(render.SetContentType(render.ContentTypeJSON))

	router.Use(cors.Handler(cors.Options{
		// AllowedOrigins:   []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins: []string{"https://*", "http://*"},
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "Accept", "Origin", "Cache-Control", "X-Requested-With"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	router.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:"+port+"/swagger/doc.json"), //The url pointing to API definition
	))

	router.Get("/api/getnotes", c.GetNotes())
	router.Get("/api/getinbox", c.GetInbox())
	router.Get("/api/getnotifications", c.GetNotifications())

	router.Get("/api/getnewnotescount", c.GetNewNotesCount())

	router.Get("/api/getlastseenid", c.GetLastSeenID())

	/**
	 * Put a user on the naughty list
	 */
	router.Post("/api/blockuser", c.BlockUser())

	/**
	 * Put a user on the follow list
	 * This is all local and will not send an event for followlist
	 */
	router.Post("/api/followuser", c.Follow())
	router.Post("/api/unfollowuser", c.Unfollow())
	router.Get("/api/getfollowed", c.GetFollowedProfiles())

	/**
	 * Bookmark events you want to keep track of
	 */
	router.Post("/api/bookmark", c.AddBookMark())
	router.Post("/api/removebookmark", c.RemoveBookMark())

	/**
	 * Relay settings
	 */
	router.Post("/api/addrelay", c.AddRelay())
	router.Post("/api/removerelay", c.RemoveRelay())
	router.Get("/api/getrelays", c.GetRelays())

	/**
	 * Sometimes it is nice to see pictures in the post and not just a link
	 */
	router.Post("/api/preview/link", c.PreviewLink())

	/**
	 * Sometimes it is nice to see pictures in the post and not just a link
	 */
	router.Post("/api/publish", c.Publish())

	/**
	 * Use meta data set and get
	 */
	router.Get("/api/getmetadata", c.GetMetaData())
	router.Post("/api/setmetadata", c.SetMetaData())
	router.Get("/api/getprofile", c.GetProfile())

	return router
}
