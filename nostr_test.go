package main

import (
	"testing"
)

func TestNostrPost(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		panic(err)
	}

	var nostr Nostr
	nostr.Cfg = cfg

	nostr.Post("This is a test")
}
