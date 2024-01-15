package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
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

func configDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		dir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, ".config"), nil
	default:
		return os.UserConfigDir()
	}
}

/**
 * Get the content of config.json file
 */
func LoadConfig() (*Config, error) {
	var cfg Config

	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "relaystore")
	fp := filepath.Join(dir, "config.json")
	os.MkdirAll(filepath.Dir(fp), 0700)

	content, err := os.ReadFile(fp)
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
