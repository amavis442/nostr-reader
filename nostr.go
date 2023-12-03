package main

import (
	"context"
	"fmt"
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
func (nostr *Nostr) Do(f func(context.Context, *nostrHandler.Relay) bool) {
	var wg sync.WaitGroup

	for _, relayUrl := range nostr.Cfg.Relays {
		wg.Add(1)

		go func(wg *sync.WaitGroup, relayUrl string) {
			defer wg.Done()
			ctx := context.WithValue(context.Background(), "relayUrl", relayUrl)
			relay, err := nostrHandler.RelayConnect(ctx, relayUrl)
			if err != nil {
				log.Println(err)
				return
			}
			if !f(ctx, relay) { // Custom function call that takes nostr.Relay as an argument. The function f will probally be an anonymous function
				ctx.Done()
			}
			relay.Close()
		}(&wg, relayUrl)
	}
	wg.Wait()
}

func (nostr *Nostr) Publish(f func(context.Context, *nostrHandler.Relay) bool) {
	var wg sync.WaitGroup

	for _, relayUrl := range nostr.Cfg.Relays {
		wg.Add(1)

		go func(wg *sync.WaitGroup, relayUrl string) {
			defer wg.Done()
			ctx := context.WithValue(context.Background(), "url", relayUrl)
			relay, err := nostrHandler.RelayConnect(ctx, relayUrl)
			if err != nil {
				log.Println(err)
				return
			}
			f(ctx, relay) // Custom function call that takes nostr.Relay as an argument. The function f will probally be an anonymous function
			fmt.Printf("published to %s\n", relayUrl)
			relay.Close()
		}(&wg, relayUrl)
	}
	wg.Wait()
}

func (nostr *Nostr) Post(content string) {
	ev := nostrHandler.Event{
		PubKey:    nostr.Cfg.Pubkey,
		CreatedAt: nostrHandler.Now(),
		Kind:      nostrHandler.KindTextNote,
		Tags:      nil,
		Content:   content,
	}

	// calling Sign sets the event ID field and the event Sig field
	ev.Sign(nostr.Cfg.Pk)

	nostr.Publish(func(ctx context.Context, relay *nostrHandler.Relay) bool {
		_, err := relay.Publish(ctx, ev)
		if err != nil {
			fmt.Println(err)
			return false
		}
		return true
	})
}

/**
 * Send a request over a websocket to get new events (notes) and after processing the events
 * try to get all the usernames metadata from who posted the note.
 */
func (nostr *Nostr) GetEvents(filter nostrHandler.Filter) {
	log.Println("Get Event data from relays")
	var m sync.Map
	var mu sync.Mutex
	nostr.Do(func(ctx context.Context, relay *nostrHandler.Relay) bool {
		mu.Lock()
		evs, err := relay.QuerySync(ctx, filter)
		if err != nil {
			mu.Unlock()
			return false
		}
		/**
		 * Deduplicate
		 * Make sure we only have 1 copy of the event even when we have multiple relays that have this event stored.
		 */
		for _, ev := range evs {
			if _, ok := m.Load(ev.ID); !ok {
				m.LoadOrStore(ev.ID, ev)
				if len(filter.IDs) == 1 {
					ctx.Done()
					break
				}
			}
		}
		mu.Unlock()
		return true
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

	nostr.GetEvents(filter)

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
	nostr.Do(func(ctx context.Context, relay *nostrHandler.Relay) bool {
		evs, err := relay.QuerySync(ctx, filter)
		if err != nil {
			return false
		}
		for _, ev := range evs {
			if _, ok := m.Load(ev.ID); !ok {
				m.LoadOrStore(ev.ID, ev)
			}
		}
		return true
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
