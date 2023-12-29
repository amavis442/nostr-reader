package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type Server struct {
	Port int64
}

type Relay struct {
	Read   bool `json:"read"`
	Write  bool `json:"write"`
	Search bool `json:"search"`
}

/**
 * Used to store the config.json file and some database related stuff for easy access
 *
 */
type Config struct {
	Database *DbConfig
	Relays   map[string]Relay
	Pubkey   string
	Npub     string
	Pk       string
	Nsec     string
	Filter   []string
	Storage  *Storage
	Server   *Server
}

/**
 * Get the content of config.json file
 */
func LoadConfig() (*Config, error) {
	var cfg Config

	content, err := os.ReadFile("./config.json")
	if err != nil {
		fmt.Println("Done", err)
		log.Println("Error when opening file: ", err)
		return nil, err
	}

	err = json.Unmarshal(content, &cfg)
	if err != nil {
		log.Println("Error during Unmarshal(): ", err)
		return nil, err
	}

	//log.Println("Content nieuw", *settings)
	// Let's print the unmarshalled data!
	fmt.Printf("dbName: %s\n", cfg.Database.Dbname)
	fmt.Printf("Pubkey: %s\n", cfg.Pubkey)
	return &cfg, nil
}
