package main

import (
	"context"
	"encoding/json"
	"errors"
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

type UserProfile struct {
	Name        string `json:"name"`
	About       string `json:"about"`
	Picture     string `json:"picture"`
	Website     string `json:"website"`
	Nip05       string `json:"nip05"`
	Lud16       string `json:"lud16"`
	DisplayName string `json:"display_name"`
	Pubkey      string `json:"pubkey"`
}

type Profile struct {
	UserProfile
	Followed bool `json:"followed"`
}

type Event struct {
	Event    *nostrHandler.Event `json:"event"`
	Profile  Profile             `json:"profile"`
	Etags    []string            `json:"etags"`
	Ptags    []string            `json:"ptags"`
	Garbage  bool                `json:"gargabe"`
	Children map[string]*Event   `json:"children"`
	Tree     int64               `json:"tree"`
	RootId   string              `json:"root_id"`
}
type RelayUrl string

var KeyUrl RelayUrl = "relayUrl"

/*
 * Please see https://github.com/mattn/algia/blob/main/main.go for the code i shamelessly copied
 *
 * Fire off calls to relays for getting new posts, user metadata etc. Each relay is operated in it's own thread
 * The f function is used to process the data we get from the relays.
 *
 * It just makes sure all available relays are called
 */
func (nostr *Nostr) Do(r Relay, f func(context.Context, *nostrHandler.Relay) bool) {
	var wg sync.WaitGroup

	for relayUrl, v := range nostr.Cfg.Relays {
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
			relay, err := nostrHandler.RelayConnect(ctx, relayUrl)
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

func (nostr *Nostr) Post(ctx context.Context, content string) (nostrHandler.Event, error) {
	ev := nostrHandler.Event{}
	ev.Tags = nostrHandler.Tags{}
	ctx, cancel := context.WithTimeout(ctx, time.Second*15) // It has 15 seconds to complete or else it will cancel itself.
	defer cancel()

	ev.PubKey = nostr.Cfg.Pubkey
	ev.CreatedAt = nostrHandler.Now()
	ev.Kind = nostrHandler.KindTextNote
	ev.Content = content

	var success int
	nostr.Do(Relay{Write: true}, func(ctx context.Context, relay *nostrHandler.Relay) bool {
		// calling Sign sets the event ID field and the event Sig field
		if err := ev.Sign(nostr.Cfg.Pk); err != nil {
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
		return nostrHandler.Event{}, errors.New("cannot post")
	}
	log.Println("Post:: Saving post: ", ev)

	nostr.Storage.SaveEvents(ctx, []*nostrHandler.Event{&ev})
	return ev, nil
}

func (nostr *Nostr) Reply(ctx context.Context, content string, event_id string) (nostrHandler.Event, error) {
	if event_id == "" {
		log.Println("Reply::Wrong function call. needs event_id since is is a reply")
		return nostrHandler.Event{}, errors.New("no event_id in call")
	}

	ev := nostrHandler.Event{}
	ev.Tags = nostrHandler.Tags{}
	var replyETags nostrHandler.Tags
	var replyEv nostrHandler.Event
	var err error

	ctx, cancel := context.WithTimeout(ctx, time.Second*15) // It has 15 seconds to complete or else it will cancel itself.
	defer cancel()

	replyEv, err = nostr.Storage.FindRawEvent(ctx, event_id)
	if err != nil {
		return nostrHandler.Event{}, err
	}
	replyETags = replyEv.Tags.GetAll([]string{"e"})

	ev.PubKey = nostr.Cfg.Pubkey
	ev.CreatedAt = nostrHandler.Now()
	ev.Kind = nostrHandler.KindTextNote
	ev.Content = content

	var hasRootTag bool = false

	// We reply to the root of the Thread
	if len(replyETags) == 0 {
		ev.Tags = ev.Tags.AppendUnique(nostrHandler.Tag{"e", replyEv.ID, "", "root"})
		ev.Tags = ev.Tags.AppendUnique(nostrHandler.Tag{"p", replyEv.PubKey})
	}

	// We reply to a reply which should have tags
	if len(replyEv.Tags) > 0 {
		for _, tag := range replyEv.Tags {
			if tag[0] == "e" && len(tag) > 2 {
				if tag[3] == "root" {
					ev.Tags = ev.Tags.AppendUnique(nostrHandler.Tag{tag[0], tag[1], tag[2], "root"})
					hasRootTag = true
				}
			}

			if tag[0] == "p" {
				ev.Tags = ev.Tags.AppendUnique(tag)
			}
		}

		if !hasRootTag && len(replyETags) > 0 {
			ev.Tags = ev.Tags.AppendUnique(nostrHandler.Tag{"e", event_id, "", "root"})
		}
		if hasRootTag && len(replyETags) > 0 {
			ev.Tags = ev.Tags.AppendUnique(nostrHandler.Tag{"e", event_id, "", "reply"})
		}
		ev.Tags = ev.Tags.AppendUnique(nostrHandler.Tag{"p", replyEv.PubKey})
	}

	var success int
	nostr.Do(Relay{Write: true}, func(ctx context.Context, relay *nostrHandler.Relay) bool {
		// calling Sign sets the event ID field and the event Sig field
		if err := ev.Sign(nostr.Cfg.Pk); err != nil {
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
		return nostrHandler.Event{}, errors.New("cannot reply")
	}

	nostr.Storage.SaveEvents(ctx, []*nostrHandler.Event{&ev})
	return ev, nil
}

/**
 * Send a request over a websocket to get new events (notes) and after processing the events
 * try to get all the usernames metadata from who posted the note.
 */
func (nostr *Nostr) GetEvents(ctx context.Context, filter nostrHandler.Filter, withProfiles bool) {
	var m sync.Map
	var mu sync.Mutex
	found := false

	nostr.Do(Relay{Read: true}, func(ctx context.Context, relay *nostrHandler.Relay) bool {
		mu.Lock()
		if found {
			mu.Unlock()
			return false
		}
		mu.Unlock()

		evs, err := relay.QuerySync(ctx, filter)
		if err != nil {
			mu.Unlock()
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

	var evs []*nostrHandler.Event
	m.Range(func(k, v any) bool {
		evs = append(evs, v.(*nostrHandler.Event))
		return true
	})

	/**
	 * Array of all the pubkeys in the event also the #p tags
	 */
	var pubkeys = make([]string, 0)
	pubkeys = nostr.Storage.SaveEvents(ctx, evs)

	if withProfiles {
		// Last but not least, try to get the user metadata
		nostr.updateProfiles(ctx, pubkeys)
	}
}

/**
 * Before we try to get events, first get the last timestamp so we do not query all the events all the time but only the lastests.
 * We do not want to spam the relays when we just synced, so wait 60 seconds before we accept a new sync
 */
func (nostr *Nostr) getEventData(ctx context.Context) {
	var createdAt int64
	var createdAtOffset int64 = time.Now().Unix() - 60

	createdAt = nostr.Storage.getLastTimeStamp(ctx)

	if createdAt < 1 {
		createdAt = createdAtOffset
	}
	if createdAt > createdAtOffset {
		fmt.Printf("Time lapse is to short for getting new data %d %d", createdAt, createdAtOffset)
		return
	}

	var timeStamp nostrHandler.Timestamp = nostrHandler.Timestamp(createdAt + 1)
	fmt.Println("Nostr Timestamp: ", timeStamp)
	filter := nostrHandler.Filter{
		Kinds: []int{nostrHandler.KindTextNote, nostrHandler.KindReaction, nostrHandler.KindArticle},
		Since: &timeStamp,
	}

	nostr.GetEvents(ctx, filter, true)
}

/**
 * Get the metadata of a bunch of Pubkeys and store them.
 */
func (nostr *Nostr) updateProfiles(ctx context.Context, pubkeys []string) {
	// Todo build check for ttl so user data is not refreshed every time.
	var tresholdTime int64 = time.Now().Unix() - 60*60*24

	pubkeys, _ = nostr.Storage.CheckProfiles(ctx, pubkeys, tresholdTime)
	if (len(pubkeys)) < 1 {
		return
	}

	filter := nostrHandler.Filter{
		Kinds:   []int{nostrHandler.KindProfileMetadata},
		Authors: pubkeys,
	}

	var m sync.Map
	nostr.Do(Relay{Read: true}, func(ctx context.Context, relay *nostrHandler.Relay) bool {
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
		evs = append(evs, v.(*nostrHandler.Event))
		return true
	})

	nostr.Storage.SaveProfiles(ctx, evs)
}

/**
 * Put a user on the naugthy list.
 */
func (nostr *Nostr) blockPubkey(ctx context.Context, user *BlockPubkey) error {
	err := nostr.Storage.BlockPubkey(ctx, user.Pubkey)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (nostr *Nostr) FollowPubkey(ctx context.Context, user *FollowPubkey) error {
	err := nostr.Storage.FollowPubkey(ctx, user.Pubkey)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (nostr *Nostr) UnfollowPubkey(ctx context.Context, user *FollowPubkey) error {
	err := nostr.Storage.UnfollowPubkey(ctx, user.Pubkey)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (nostr *Nostr) GetMetaData(ctx context.Context) (nostrHandler.Event, error) {
	pubkey := nostr.Cfg.Pubkey

	filter := nostrHandler.Filter{
		Kinds:   []int{nostrHandler.KindProfileMetadata},
		Authors: []string{pubkey},
		Limit:   1,
	}

	var m sync.Map
	nostr.Do(Relay{Read: true}, func(ctx context.Context, relay *nostrHandler.Relay) bool {
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
		event := v.(*nostrHandler.Event)
		return *event, nil
	}

	return nostrHandler.Event{}, nil
}

func (nostr *Nostr) SetMetaData(ctx context.Context, user *UserProfile) error {
	ev := nostrHandler.Event{}
	ev.Tags = nostrHandler.Tags{}

	ev.PubKey = nostr.Cfg.Pubkey
	ev.CreatedAt = nostrHandler.Now()
	ev.Kind = nostrHandler.KindProfileMetadata
	c, err := json.Marshal(*user)
	if err != nil {
		log.Println(err)
		return err
	}
	ev.Content = string(c)

	fmt.Println(ev)
	var success int
	nostr.Do(Relay{Write: true}, func(ctx context.Context, relay *nostrHandler.Relay) bool {
		// calling Sign sets the event ID field and the event Sig field
		if err := ev.Sign(nostr.Cfg.Pk); err != nil {
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
