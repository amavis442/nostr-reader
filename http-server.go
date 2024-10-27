package main

import (
	"fmt"
	"log"
	"mime"
	"net/http"

	"github.com/go-chi/chi"
)

type ServerConfig struct {
	Port int64
}

type HttpServer struct {
	DevMode  bool
	Server   *ServerConfig
	Database *Storage
	Nostr    *Wrapper
	Router   *chi.Mux
}

func (s *HttpServer) Routes(c *Controller, port string) {
	s.Router = routes(c, port)
}

// @title Nostr Reader API
// @version 1.0
// @description Nostr Reader Api.
// @termsOfService http://swaggerouter.io/terms/

// @contact.name API Support
// @contact.url http://www.swaggerouter.io/support
// @contact.email support@swaggerouter.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /
func (s *HttpServer) Start() {
	var c Controller
	c.Pubkey = s.Nostr.Cfg.PubKey
	c.Db = s.Database
	c.Nostr = s.Nostr

	var port string = "8080"
	if s.Server.Port > 0 {
		port = fmt.Sprint(s.Server.Port)
	}

	s.Routes(&c, port)
	//router := routes(&c, port)

	// Windows may be missing this
	mime.AddExtensionType(".js", "application/javascript")

	fmt.Println("Server running: http://localhost:" + port)

	err := http.ListenAndServe(":"+port, s.Router)
	if err != nil {
		log.Println("Could not start http server on this port: " + port)
		log.Fatal(err)
	}
}
