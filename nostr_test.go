package main

import (
	"context"
	"testing"
)

func TestNostrPost(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		panic(err)
	}

	var nostr Nostr
	nostr.Cfg = cfg

	nostr.Post(context.Background(), "This is a test", "")
}
