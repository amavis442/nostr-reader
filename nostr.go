package main

import (
	"context"
	"log"
	"sync"
	"time"

	nostrHandler "github.com/nbd-wtf/go-nostr"
)

type Nostr struct {
	Storage *Storage
	Cfg     *Config
}

type Relay struct {
	Relay *nostrHandler.Relay
	Url   string
	Read  bool
	Write bool
}

/*
 * Please see https://github.com/mattn/algia/blob/main/main.go for the code i shamelessly copied
 *
 * Fire off calls to relays for getting new posts, user metadata etc. Each relay is operated in it's own thread
 * The f function is used to process the data we get from the relays.
 *
 * It just makes sure all available relays are called
 */
func (nostr *Nostr) Do(f func(*nostrHandler.Relay)) {
	var wg sync.WaitGroup

	for _, v := range nostr.Cfg.Relays {
		wg.Add(1)

		go func(wg *sync.WaitGroup, v string) {
			defer wg.Done()
			ctx := context.WithValue(context.Background(), "url", v)
			relay, err := nostrHandler.RelayConnect(ctx, v)
			if err != nil {
				log.Println(err)
				return
			}
			f(relay) // Custom function call that takes nostr.Relay as an argument. The function f will probally be an anonymous function
			relay.Close()
		}(&wg, v)
	}
	wg.Wait()
}

/**
 * Send a request over a websocket to get new events (notes) and after processing the events
 * try to get all the usernames metadata from who posted the note.
 */
func (nostr *Nostr) getEvents(filter nostrHandler.Filter) {
	log.Println("Get Event data from relays")
	var m sync.Map
	nostr.Do(func(relay *nostrHandler.Relay) {
		evs, err := relay.QuerySync(context.Background(), filter)
		if err != nil {
			return
		}
		/**
		 * Deduplicate
		 * Make sure we only have 1 copy of the event even when we have multiple relays that have this event stored.
		 */
		for _, ev := range evs {
			if _, ok := m.Load(ev.ID); !ok {
				m.LoadOrStore(ev.ID, ev)
			}
		}
	})

	/**
	 * Turn the sync map into an array we can process
	 */
	var evs []*nostrHandler.Event
	m.Range(func(k, v any) bool {
		log.Println(k)
		evs = append(evs, v.(*nostrHandler.Event))
		return true
	})

	/**
	 * Array of all the pubkeys in the event also the #p tags
	 */
	var pubkeys = make([]string, 0)
	pubkeys = nostr.Storage.SaveEvents(evs)
	for i, pubkey := range pubkeys {
		log.Println(i, pubkey)
	}

	// Last but not least, try to get the user metadata
	defer nostr.updateProfiles(pubkeys)

	defer func() {
		log.Println("Done receiving and closed ralay connections")
	}()
}

/**
 * Before we try to get events, first get the last timestamp so we do not query all the events all the time but only the lastests.
 * We do not want to spam the relays when we just synced, so wait 60 seconds before we accept a new sync
 */
func (nostr *Nostr) getEventData() {
	var createdAt int64
	var createdAtOffset int64 = time.Now().Unix() - 60

	//row := cfg.Storage.Db.QueryRow("SELECT MAX(created_at) as MaxCreated FROM events")
	//row.Scan(&createdAt)
	createdAt = nostr.Storage.getLastTimeStamp()

	log.Println(createdAt)
	if createdAt < 1 {
		createdAt = createdAtOffset
	}
	if createdAt > createdAtOffset {
		log.Printf("Time lapse is to short for getting new data %d %d", createdAt, createdAtOffset)
		return
	}

	var timeStamp nostrHandler.Timestamp = nostrHandler.Timestamp(createdAt + 1)
	filter := nostrHandler.Filter{
		Kinds: []int{nostrHandler.KindTextNote, nostrHandler.KindReaction, nostrHandler.KindArticle},
		Since: &timeStamp,
	}

	nostr.getEvents(filter)

	defer func() {
		log.Println("Closing shop")
	}()
}

/**
 * Get the metadata of a bunch of Pubkeys and store them.
 */
func (nostr *Nostr) updateProfiles(pubkeys []string) {
	filter := nostrHandler.Filter{
		Kinds:   []int{nostrHandler.KindProfileMetadata},
		Authors: pubkeys,
	}

	log.Println("Get user data from relays")
	var m sync.Map
	nostr.Do(func(relay *nostrHandler.Relay) {
		evs, err := relay.QuerySync(context.Background(), filter)
		if err != nil {
			return
		}
		for _, ev := range evs {
			if _, ok := m.Load(ev.ID); !ok {
				m.LoadOrStore(ev.ID, ev)
			}
		}
	})

	var evs []*nostrHandler.Event
	m.Range(func(k, v any) bool {
		log.Println(k)
		evs = append(evs, v.(*nostrHandler.Event))
		return true
	})

	nostr.Storage.SaveProfiles(evs)
	log.Println("Done for profiles")
}

/**
 * Put a user on the naugthy list.
 */
func (nostr *Nostr) blockPubkey(user *BlockPubkey) {
	err := nostr.Storage.BlockPubkey(user.Pubkey)
	if err != nil {
		log.Println(err)
	}
}
