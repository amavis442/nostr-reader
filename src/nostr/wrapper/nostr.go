package wrapper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

type Relay struct {
	Read   bool `json:"read"`
	Write  bool `json:"write"`
	Search bool `json:"search"`
}

type Config struct {
	Relays     map[string]Relay
	PubKey     string
	Npub       string
	PrivateKey string
	Nsec       string
	Filter     []string
}

type Profile struct {
	Name        string `json:"name"`
	About       string `json:"about"`
	Picture     string `json:"picture"`
	Website     string `json:"website"`
	Nip05       string `json:"nip05"`
	Lud16       string `json:"lud16"`
	DisplayName string `json:"display_name"`
	Pubkey      string `json:"pubkey"`
}

type RelayUrl string

var KeyUrl RelayUrl = "relayUrl"

type NostrWrapper struct {
	Cfg Config
}

func (nostrWrapper *NostrWrapper) SetConfig(cfg *Config) {
	nostrWrapper.Cfg = *cfg
}

/*
 * Please see https://github.com/mattn/algia/blob/main/main.go for the code i shamelessly copied
 *
 * Fire off calls to relays for getting new posts, user metadata etc. Each relay is operated in it's own thread
 * The f function is used to process the data we get from the relays.
 *
 * It just makes sure all available relays are called
 */
func (nostrWrapper *NostrWrapper) Do(r Relay, f func(context.Context, *nostr.Relay) bool) {
	var wg sync.WaitGroup

	for relayUrl, v := range nostrWrapper.Cfg.Relays {
		if r.Write && !v.Write {
			continue
		}
		if r.Search && !v.Search {
			continue
		}
		if !r.Write && !v.Read {
			continue
		}
		wg.Add(1)

		go func(wg *sync.WaitGroup, relayUrl string, v Relay) {
			defer wg.Done()
			ctx := context.WithValue(context.Background(), KeyUrl, relayUrl)
			relay, err := nostr.RelayConnect(ctx, relayUrl)
			if err != nil {
				log.Println(err)
				return
			}

			if !f(ctx, relay) { // Custom function call that takes nostr.Relay as an argument. The function f will probally be an anonymous function
				ctx.Done()
			}

			relay.Close()
		}(&wg, relayUrl, v)
	}
	wg.Wait()
}

func (nostrWrapper *NostrWrapper) Post(ctx context.Context, content string) (nostr.Event, error) {
	ev := nostr.Event{}
	ev.Tags = nostr.Tags{}

	ev.PubKey = nostrWrapper.Cfg.PubKey
	ev.CreatedAt = nostr.Now()
	ev.Kind = nostr.KindTextNote
	ev.Content = content

	var success int
	nostrWrapper.Do(Relay{Write: true}, func(ctx context.Context, relay *nostr.Relay) bool {
		// calling Sign sets the event ID field and the event Sig field
		if err := ev.Sign(nostrWrapper.Cfg.PrivateKey); err != nil {
			return false
		}

		err := relay.Publish(ctx, ev)
		if err != nil {
			log.Println("Post:: ", err)
			return false
		}
		log.Println("Post:: Publish to: [", relay.URL, "], Event data: ", ev)
		success += 1
		return true
	})

	if success == 0 {
		log.Println("Post:: cannot post")
		return nostr.Event{}, errors.New("cannot post")
	}
	log.Println("Post:: Saving post: ", ev)

	return ev, nil
}

func (nostrWrapper *NostrWrapper) Reply(ctx context.Context, content string, replyEv nostr.Event) (nostr.Event, error) {
	if replyEv.ID == "" {
		log.Println("Reply::Wrong function call. needs event_id since it is a reply")
		return nostr.Event{}, errors.New("no reply event in call")
	}

	ev := nostr.Event{}
	ev.Tags = nostr.Tags{}
	replyETags := replyEv.Tags.GetAll([]string{"e"})

	ev.PubKey = nostrWrapper.Cfg.PubKey
	ev.CreatedAt = nostr.Now()
	ev.Kind = nostr.KindTextNote
	ev.Content = content

	var hasRootTag bool = false

	// We reply to the root of the Thread
	if len(replyETags) == 0 {
		ev.Tags = ev.Tags.AppendUnique(nostr.Tag{"e", replyEv.ID, "", "root"})
		ev.Tags = ev.Tags.AppendUnique(nostr.Tag{"p", replyEv.PubKey})
	}

	// We reply to a reply which should have tags
	if len(replyEv.Tags) > 0 {
		for _, tag := range replyEv.Tags {
			if tag[0] == "e" && len(tag) > 2 {
				if tag[3] == "root" {
					ev.Tags = ev.Tags.AppendUnique(nostr.Tag{tag[0], tag[1], tag[2], "root"})
					hasRootTag = true
				}
			}

			if tag[0] == "p" {
				ev.Tags = ev.Tags.AppendUnique(tag)
			}
		}

		if !hasRootTag && len(replyETags) > 0 {
			ev.Tags = ev.Tags.AppendUnique(nostr.Tag{"e", replyEv.ID, "", "root"})
		}
		if hasRootTag && len(replyETags) > 0 {
			ev.Tags = ev.Tags.AppendUnique(nostr.Tag{"e", replyEv.ID, "", "reply"})
		}
		ev.Tags = ev.Tags.AppendUnique(nostr.Tag{"p", replyEv.PubKey})
	}

	var success int
	nostrWrapper.Do(Relay{Write: true}, func(ctx context.Context, relay *nostr.Relay) bool {
		// calling Sign sets the event ID field and the event Sig field
		if err := ev.Sign(nostrWrapper.Cfg.PrivateKey); err != nil {
			return false
		}

		err := relay.Publish(ctx, ev)
		if err != nil {
			log.Println("Reply:: ", err)
			return false
		}
		log.Println("Reply:: publish to: [", relay.URL, "], Event data: ", ev)
		success += 1
		return true
	})

	if success == 0 {
		log.Println("Reply:: cannot reply")
		return nostr.Event{}, errors.New("cannot reply")
	}

	return ev, nil
}

/**
 * Send a request over a websocket to get new events (notes) and after processing the events
 * try to get all the usernames metadata from who posted the note.
 */
func (nostrWrapper *NostrWrapper) GetEvents(ctx context.Context, filter nostr.Filter) []*nostr.Event {
	var m sync.Map
	var mu sync.Mutex
	found := false

	nostrWrapper.Do(Relay{Read: true}, func(ctx context.Context, relay *nostr.Relay) bool {
		mu.Lock()
		if found {
			mu.Unlock()
			return false
		}
		mu.Unlock()

		evs, err := relay.QuerySync(ctx, filter)
		if err != nil {
			return true
		}
		/**
		 * Deduplicate
		 * Make sure we only have 1 copy of the event even when we have multiple relays that have this event stored.
		 */
		for _, ev := range evs {
			if _, ok := m.Load(ev.ID); !ok {
				m.LoadOrStore(ev.ID, ev)
				if len(filter.IDs) == 1 {
					mu.Lock()
					found = true
					ctx.Done()
					mu.Unlock()
					break
				}
			}
		}
		return true
	})

	var evs []*nostr.Event
	m.Range(func(k, v any) bool {
		evs = append(evs, v.(*nostr.Event))
		return true
	})

	return evs
}

/**
 * Before we try to get events, first get the last timestamp so we do not query all the events all the time but only the lastests.
 * We do not want to spam the relays when we just synced, so wait 60 seconds before we accept a new sync
 */
func (nostrWrapper *NostrWrapper) GetEventData(ctx context.Context, createdAt int64, withOffset bool) nostr.Filter {
	var createdAtOffset int64 = time.Now().Unix() - 60
	if createdAt < 1 {
		createdAt = createdAtOffset
	}
	if createdAt > createdAtOffset && withOffset {
		fmt.Printf("Time lapse is to short for getting new data %d %d", createdAt, createdAtOffset)
		return nostr.Filter{}
	}

	var timeStamp nostr.Timestamp = nostr.Timestamp(createdAt + 1)
	fmt.Println("Nostr Timestamp: ", timeStamp)
	filter := nostr.Filter{
		Kinds: []int{nostr.KindTextNote, nostr.KindReaction, nostr.KindArticle, nostr.KindDeletion, nostr.KindProfileMetadata},
		Since: &timeStamp,
	}

	return filter
}

/**
 * Get the metadata of a bunch of Pubkeys and store them.
 */
func (nostrWrapper *NostrWrapper) UpdateProfiles(ctx context.Context, pubkeys []string) []*nostr.Event {
	if (len(pubkeys)) < 1 {
		return nil
	}

	filter := nostr.Filter{
		Kinds:   []int{nostr.KindProfileMetadata},
		Authors: pubkeys,
	}

	var m sync.Map
	nostrWrapper.Do(Relay{Read: true}, func(ctx context.Context, relay *nostr.Relay) bool {
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

	var evs []*nostr.Event
	m.Range(func(k, v any) bool {
		evs = append(evs, v.(*nostr.Event))
		return true
	})

	return evs
}

func (nostrWrapper *NostrWrapper) GetMetaData(ctx context.Context) (nostr.Event, error) {
	pubkey := nostrWrapper.Cfg.PubKey

	filter := nostr.Filter{
		Kinds:   []int{nostr.KindProfileMetadata},
		Authors: []string{pubkey},
		Limit:   1,
	}

	var m sync.Map
	nostrWrapper.Do(Relay{Read: true}, func(ctx context.Context, relay *nostr.Relay) bool {
		evs, err := relay.QuerySync(ctx, filter)
		if err != nil {
			return false
		}
		for _, ev := range evs {
			if _, ok := m.Load(ev.PubKey); !ok {
				m.LoadOrStore(ev.PubKey, ev)
			}
		}
		return true
	})

	if v, ok := m.Load(pubkey); ok {
		event := v.(*nostr.Event)
		return *event, nil
	}

	return nostr.Event{}, nil
}

func (nostrWrapper *NostrWrapper) SetMetaData(ctx context.Context, user *Profile) error {
	ev := nostr.Event{}
	ev.Tags = nostr.Tags{}

	ev.PubKey = nostrWrapper.Cfg.PubKey
	ev.CreatedAt = nostr.Now()
	ev.Kind = nostr.KindProfileMetadata
	c, err := json.Marshal(*user)
	if err != nil {
		log.Println(err)
		return err
	}
	ev.Content = string(c)

	fmt.Println(ev)
	var success int
	nostrWrapper.Do(Relay{Write: true}, func(ctx context.Context, relay *nostr.Relay) bool {
		// calling Sign sets the event ID field and the event Sig field
		if err := ev.Sign(nostrWrapper.Cfg.PrivateKey); err != nil {
			return false
		}

		err := relay.Publish(ctx, ev)
		if err != nil {
			fmt.Println(err)
			log.Println(err)
			return false
		}
		success += 1
		return true
	})

	if success == 0 {
		return errors.New("cannot send profile metadata")
	}

	return nil
}
