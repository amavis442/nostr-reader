package wrapper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
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
 * Fire off calls to relays for getting new posts, user metadata etc. Each relay is operated in it's own thread.
 * The f function is used to process to get data from the relays and return it for further processing.
 *
 */
func (nostrWrapper *NostrWrapper) Do(ctx context.Context, r Relay, f func(context.Context, *nostr.Relay) bool) {
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

			relay, err := nostr.RelayConnect(ctx, relayUrl)
			if err != nil {
				log.Println("Can't connect to relay: ", relay.URL)
				return
			}

			if !f(ctx, relay) {
				ctx.Done()
			}

			relay.Close()
		}(&wg, relayUrl, v)
	}
	wg.Wait()
}

/*
 * Creates a new message
 */
func (nostrWrapper *NostrWrapper) DoPost(content string) (nostr.Event, error) {
	var err error
	ev := nostr.Event{}
	ev.Tags = nostr.Tags{}
	ev.PubKey, err = nostr.GetPublicKey(nostrWrapper.Cfg.PrivateKey)
	if err != nil {
		return nostr.Event{}, err
	}
	ev.CreatedAt = nostr.Now()
	ev.Kind = nostr.KindTextNote
	ev.Content = content

	if err := ev.Sign(nostrWrapper.Cfg.PrivateKey); err != nil {
		return nostr.Event{}, err
	}

	return ev, nil
}

/*
 * Creates a reply message
 */
func (nostrWrapper *NostrWrapper) DoReply(content string, replyEv nostr.Event) (nostr.Event, error) {
	if replyEv.ID == "" {
		log.Println("Reply::Wrong function call. needs event_id since it is a reply")
		return nostr.Event{}, errors.New("no reply event in call")
	}
	var err error
	ev := nostr.Event{}
	ev.Tags = nostr.Tags{}
	replyETags := replyEv.Tags.GetAll([]string{"e"})

	ev.PubKey, err = nostr.GetPublicKey(nostrWrapper.Cfg.PrivateKey)
	if err != nil {
		return nostr.Event{}, err
	}
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
		// For the clients that do not use the root/reply tags which is a rubbish
		if !hasRootTag && len(replyETags) > 0 {
			ev.Tags = ev.Tags.AppendUnique(nostr.Tag{"e", replyETags[0][1], "", "root"})
			hasRootTag = true
		}
		if hasRootTag && len(replyETags) > 0 {
			ev.Tags = ev.Tags.AppendUnique(nostr.Tag{"e", replyEv.ID, "", "reply"})
		}
		ev.Tags = ev.Tags.AppendUnique(nostr.Tag{"p", replyEv.PubKey})
	}

	if err := ev.Sign(nostrWrapper.Cfg.PrivateKey); err != nil {
		return nostr.Event{}, err
	}

	return ev, nil
}

func (nostrWrapper *NostrWrapper) BroadCast(ctx context.Context, ev nostr.Event) (bool, error) {
	var success atomic.Int64
	nostrWrapper.Do(ctx, Relay{Write: true}, func(ctx context.Context, relay *nostr.Relay) bool {
		err := relay.Publish(ctx, ev)
		if err != nil {
			log.Println("broadcast:: ", relay.URL, err)
		} else {
			success.Add(1)
		}
		log.Println("broadcast to: [", relay.URL, "], event data: ", ev)
		return true
	})

	if success.Load() == 0 {
		log.Println("cannot broadcast")
		return false, errors.New("cannot Broadcast")
	}

	return true, nil
}

/**
 * Send a request over a websocket to get new events (notes) and make sure we only have 1 copy of that,
 * even when it is stored on many relays.
 */
func (nostrWrapper *NostrWrapper) GetEvents(ctx context.Context, filter nostr.Filter) []*nostr.Event {
	var m sync.Map
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	nostrWrapper.Do(ctx, Relay{Read: true}, func(ctx context.Context, relay *nostr.Relay) bool {
		evs, err := relay.QuerySync(ctx, filter)
		log.Println("Connecting to:", relay.URL)
		if err != nil {
			return true
		}
		for _, ev := range evs {
			if _, ok := m.Load(ev.ID); !ok {
				log.Println(relay.URL, "::", ev.CreatedAt.Time().UTC())
				log.Println("Kind:", ev.Kind, "Event ID:", ev.ID)
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

/**
 * Before we try to get events, first get the last timestamp so we do not query all the events all the time but only the lastests.
 * We do not want to spam the relays when we just synced, so wait 60 seconds before we accept a new sync
 */
func (nostrWrapper *NostrWrapper) GetEventData(createdAt int64, withOffset bool) nostr.Filter {
	var createdAtOffset int64 = time.Now().Unix() - 60
	if createdAt < 1 {
		createdAt = createdAtOffset
	}
	if createdAt > createdAtOffset && withOffset {
		log.Printf("Time lapse is to short for getting new data %d %d", createdAt, createdAtOffset)
		return nostr.Filter{}
	}
	var timeStamp nostr.Timestamp = nostr.Timestamp(createdAt + 1)
	//var untilTimeStamp nostr.Timestamp = nostr.Timestamp(createdAt + 60*60*1000)
	log.Println("Nostr Timestamp: ", timeStamp.Time().UTC())
	filter := nostr.Filter{
		Kinds: []int{nostr.KindTextNote, nostr.KindReaction, nostr.KindArticle, nostr.KindDeletion, nostr.KindProfileMetadata},
		Since: &timeStamp,
		Limit: 1000,
	}

	log.Println(filter)
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
	nostrWrapper.Do(ctx, Relay{Read: true}, func(ctx context.Context, relay *nostr.Relay) bool {
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
	nostrWrapper.Do(ctx, Relay{Read: true}, func(ctx context.Context, relay *nostr.Relay) bool {
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

func (nostrWrapper *NostrWrapper) DoPublishMetaData(ctx context.Context, user *Profile) error {
	var err error
	ev := nostr.Event{}
	ev.Tags = nostr.Tags{}

	ev.PubKey, err = nostr.GetPublicKey(nostrWrapper.Cfg.PrivateKey)
	if err != nil {
		log.Println(err)
		return err
	}
	ev.CreatedAt = nostr.Now()
	ev.Kind = nostr.KindProfileMetadata
	c, err := json.Marshal(*user)
	if err != nil {
		log.Println(err)
		return err
	}
	ev.Content = string(c)
	if err := ev.Sign(nostrWrapper.Cfg.PrivateKey); err != nil {
		return err
	}

	fmt.Println(ev)
	var success atomic.Int64
	nostrWrapper.Do(ctx, Relay{Write: true}, func(ctx context.Context, relay *nostr.Relay) bool {
		err := relay.Publish(ctx, ev)
		if err != nil {
			log.Println(relay.URL, err)
		} else {
			success.Add(1)
		}
		return true
	})

	if success.Load() == 0 {
		return errors.New("cannot send profile metadata")
	}

	return nil
}
