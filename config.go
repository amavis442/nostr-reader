package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

/**
 * Used to store the config.json file and some database related stuff for easy access
 *
 */
type Config struct {
	Database *DbConfig
	Relays   []string
	Pubkey   string
	Npub     string
	Pk       string
	Nsec     string
	Filter   []string
	Storage  *Storage
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
	log.Printf("dbName: %s\n", cfg.Database.Dbname)
	log.Printf("Pubkey: %s\n", cfg.Pubkey)
	return &cfg, nil
}
