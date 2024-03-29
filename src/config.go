package main

import (
	"amavis442/nostr-reader/database"
	"amavis442/nostr-reader/nostr/wrapper"
	nostrWrapper "amavis442/nostr-reader/nostr/wrapper"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

type Server struct {
	Port     int64
	Frontend string
	Interval int64
}

/**
 * Used to store the config.json file and some database related stuff for easy access
 *
 */
type Config struct {
	Database *database.DbConfig
	nostrWrapper.Config
	Server *Server
	Env    string
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
	dir = filepath.Join(dir, "nostr-reader")
	fp := filepath.Join(dir, "config.json")
	os.MkdirAll(filepath.Dir(fp), 0700)

	content, err := os.ReadFile(fp)
	if err != nil {
		fmt.Println("Done", err)
		log.Fatal("Error when opening file: ", err)
	}

	err = json.Unmarshal(content, &cfg)
	if err != nil {
		log.Fatal("Error during Unmarshal(): ", err)
	}

	if cfg.PrivateKey == "" {
		log.Println("You need to add your private key. This key will never be transmitted and stays local")
		os.Exit(0)
	}

	var pubKey string
	if cfg.PrivateKey[:4] == "nsec" {
		if _, s, err := nip19.Decode(cfg.PrivateKey); err == nil {
			if pubKey, err = nostr.GetPublicKey(s.(string)); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		pubKey, _ = nostr.GetPublicKey(cfg.PrivateKey)
	}
	cfg.PubKey = pubKey
	cfg.Nsec, _ = nip19.EncodePrivateKey(cfg.PrivateKey)
	cfg.Npub, _ = nip19.EncodePublicKey(pubKey)

	return &cfg, nil
}

func UpdateRelays(cfg *nostrWrapper.Config, relays []database.Relay) {
	cfg.Relays = make(map[string]nostrWrapper.Relay, 0)

	for _, relay := range relays {
		cfg.Relays[relay.Url] = wrapper.Relay{Read: relay.Read, Write: relay.Write, Search: relay.Search}
	}
}
