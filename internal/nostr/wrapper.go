package nostr

import (
	"amavis442/nostr-reader/internal/db"
	"amavis442/nostr-reader/internal/logger"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

type WrapperConfig struct {
	Relays     map[string]db.Relay
	PubKey     string
	Npub       string
	PrivateKey string
	Nip05      string
	Nsec       string
	Filter     []string
}

type RelayUrl string

var KeyUrl RelayUrl = "relayUrl"

type Wrapper struct {
	Cfg WrapperConfig
}

func (wrapper *Wrapper) SetConfig(cfg *WrapperConfig) {
	wrapper.Cfg = *cfg
}

func (wrapper *Wrapper) GetConfig() *WrapperConfig {
	return &wrapper.Cfg
}

/*
 * Please see https://github.com/mattn/algia/blob/main/main.go for the code i shamelessly copied
 *
 * Fire off calls to relays for getting new posts, user metadata etc. Each relay is operated in it's own thread.
 * The f function is used to process to get data from the relays and return it for further processing.
 *
 */
func (wrapper *Wrapper) Do(ctx context.Context, r db.Relay, f func(context.Context, *nostr.Relay) bool) {
	var wg sync.WaitGroup
	for relayUrl, v := range wrapper.Cfg.Relays {
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

		go func(wg *sync.WaitGroup, relayUrl string, v db.Relay) {
			defer wg.Done()

			relay, err := nostr.RelayConnect(ctx, relayUrl)
			if err != nil {
				slog.Info("can't connect to relay: " + relay.URL)
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
func (wrapper *Wrapper) DoPost(content string) (db.Event, error) {
	var err error
	ev := db.Event{}
	ev.Event = &nostr.Event{}
	ev.Event.Tags = nostr.Tags{}
	ev.Event.PubKey, err = nostr.GetPublicKey(wrapper.Cfg.PrivateKey)
	if err != nil {
		return db.Event{}, err
	}
	ev.Event.CreatedAt = nostr.Now()
	ev.Event.Kind = nostr.KindTextNote
	ev.Event.Content = content

	if err := ev.Event.Sign(wrapper.Cfg.PrivateKey); err != nil {
		return db.Event{}, err
	}

	return ev, nil
}

/*
 * Creates a reply message
 */
func (wrapper *Wrapper) DoReply(content string, replyEv db.Event) (db.Event, error) {
	if replyEv.Event.ID == "" {
		log.Println("Reply::Wrong function call. needs event_id since it is a reply")
		return db.Event{}, errors.New("no reply event in call")
	}
	var err error
	ev := db.Event{}
	ev.Event = &nostr.Event{}
	ev.Event.Tags = nostr.Tags{}
	replyETags := replyEv.Event.Tags.GetAll([]string{"e"})

	ev.Event.PubKey, err = nostr.GetPublicKey(wrapper.Cfg.PrivateKey)
	if err != nil {
		return db.Event{}, err
	}
	ev.Event.CreatedAt = nostr.Now()
	ev.Event.Kind = nostr.KindTextNote
	ev.Event.Content = content

	var hasRootTag bool = false

	// We reply to the root of the Thread
	if len(replyETags) == 0 {
		ev.Event.Tags = ev.Event.Tags.AppendUnique(nostr.Tag{"e", replyEv.Event.ID, "", "root"})
		ev.Event.Tags = ev.Event.Tags.AppendUnique(nostr.Tag{"p", replyEv.Event.PubKey})
	}

	// We reply to a reply which should have tags
	if len(replyEv.Event.Tags) > 0 {
		for _, tag := range replyEv.Event.Tags {
			if tag[0] == "e" && len(tag) > 2 {
				if tag[3] == "root" {
					ev.Event.Tags = ev.Event.Tags.AppendUnique(nostr.Tag{tag[0], tag[1], tag[2], "root"})
					hasRootTag = true
				}
			}

			if tag[0] == "p" {
				ev.Event.Tags = ev.Event.Tags.AppendUnique(tag)
			}
		}
		// For the clients that do not use the root/reply tags which is a rubbish
		if !hasRootTag && len(replyETags) > 0 {
			ev.Event.Tags = ev.Event.Tags.AppendUnique(nostr.Tag{"e", replyETags[0][1], "", "root"})
			hasRootTag = true
		}
		if hasRootTag && len(replyETags) > 0 {
			ev.Event.Tags = ev.Event.Tags.AppendUnique(nostr.Tag{"e", replyEv.Event.ID, "", "reply"})
		}
		ev.Event.Tags = ev.Event.Tags.AppendUnique(nostr.Tag{"p", replyEv.Event.PubKey})
	}

	if err := ev.Event.Sign(wrapper.Cfg.PrivateKey); err != nil {
		return db.Event{}, err
	}

	return ev, nil
}

func (wrapper *Wrapper) BroadCast(ctx context.Context, ev db.Event) (bool, error) {
	var success atomic.Int64
	wrapper.Do(ctx, db.Relay{Write: true}, func(ctx context.Context, relay *nostr.Relay) bool {
		err := relay.Publish(ctx, *ev.Event)
		if err != nil {
			slog.Error(logger.GetCallerInfo(1), "broadcast", relay.URL, "error", err.Error())
			wrapper.Cfg.Relays[relay.URL] = db.Relay{Write: false}
		} else {
			success.Add(1)
		}
		slog.Info(logger.GetCallerInfo(1), "broadcast", relay.URL, "data", ev.Event)
		return true
	})

	if success.Load() == 0 {
		slog.Warn(logger.GetCallerInfo(1) + " cannot broadcast")
		return false, errors.New("cannot Broadcast")
	}

	return true, nil
}

/**
 * Send a request over a websocket to get new events (notes) and make sure we only have 1 copy of that,
 * even when it is stored on many relays.
 */
func (wrapper *Wrapper) GetEvents(ctx context.Context, filter nostr.Filter) []*db.Event {
	var m sync.Map
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	wrapper.Do(ctx, db.Relay{Read: true}, func(ctx context.Context, relay *nostr.Relay) bool {
		evs, err := relay.QuerySync(ctx, filter)
		slog.Info(fmt.Sprintf("connecting to: %s", relay.URL))
		if err != nil {
			return true
		}
		for _, ev := range evs {
			resultEv, ok := m.Load(ev.ID)
			if !ok {
				//log.Println(relay.URL, "::", ev.CreatedAt.Time().UTC())
				//log.Println("Kind:", ev.Kind, "Event ID:", ev.ID)

				myEvent := &db.Event{}
				myEvent.Event = &nostr.Event{}
				myEvent.Event = ev
				myEvent.Urls = append(myEvent.Urls, relay.URL)

				m.LoadOrStore(ev.ID, myEvent)
			}
			if ok && resultEv != nil { // Event already exists but i want to store of all relays where this can be found.
				existingEv := resultEv.(*db.Event)
				existingEv.Urls = append(existingEv.Urls, relay.URL)
				m.LoadOrStore(existingEv.Event.ID, existingEv)
			}
		}

		return true
	})

	var evs []*db.Event
	m.Range(func(k, v any) bool {
		event := v.(*db.Event)
		if event.Event.Kind == 1 && len(event.Event.Content) > 0 {
			evs = append(evs, event)
		}
		if event.Event.Kind > 1 {
			evs = append(evs, event)
		}
		return true
	})

	return evs
}

/**
 * Before we try to get events, first get the last timestamp so we do not query all the events all the time but only the lastests.
 * We do not want to spam the relays when we just synced, so wait 60 seconds before we accept a new sync
 */
func (wrapper *Wrapper) GetEventData(createdAt int64, withOffset bool) nostr.Filter {
	var createdAtOffset int64 = time.Now().Unix() - 60
	if createdAt < 1 {
		createdAt = createdAtOffset
	}
	if createdAt > createdAtOffset && withOffset {
		log.Printf("Time lapse is to short for getting new data %d %d", createdAt, createdAtOffset)
		return nostr.Filter{}
	}
	var timeStamp nostr.Timestamp = nostr.Timestamp(createdAt + 1)

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
func (wrapper *Wrapper) UpdateProfiles(ctx context.Context, pubkeys []string) []*db.Event {
	if (len(pubkeys)) < 1 {
		return nil
	}

	filter := nostr.Filter{
		Kinds:   []int{nostr.KindProfileMetadata},
		Authors: pubkeys,
	}

	var m sync.Map
	wrapper.Do(ctx, db.Relay{Read: true}, func(ctx context.Context, relay *nostr.Relay) bool {
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

	var evs []*db.Event
	m.Range(func(k, v any) bool {
		var event db.Event
		event.Event = &nostr.Event{}
		event.Event = v.(*nostr.Event)

		evs = append(evs, &event)
		return true
	})

	return evs
}

func (wrapper *Wrapper) GetMetaData(ctx context.Context) (db.Event, error) {
	pubkey := wrapper.Cfg.PubKey

	filter := nostr.Filter{
		Kinds:   []int{nostr.KindProfileMetadata},
		Authors: []string{pubkey},
		Limit:   1,
	}

	var m sync.Map
	wrapper.Do(ctx, db.Relay{Read: true}, func(ctx context.Context, relay *nostr.Relay) bool {
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
		var event db.Event
		event.Event = &nostr.Event{}
		event.Event = v.(*nostr.Event)
		return event, nil
	}

	return db.Event{}, nil
}

type NostrProfile struct {
	Pubkey      string
	Name        string
	About       string
	Picture     string
	Website     string
	Nip05       string
	Lud16       string
	DisplayName string
}

func (wrapper *Wrapper) DoPublishMetaData(ctx context.Context, user *db.Profile) error {
	var err error
	ev := nostr.Event{}
	ev.Tags = nostr.Tags{}

	ev.PubKey, err = nostr.GetPublicKey(wrapper.Cfg.PrivateKey)
	if err != nil {
		log.Println(err)
		return err
	}

	profile := &NostrProfile{
		Pubkey:      user.Pubkey,
		Name:        user.Name.String,
		About:       user.About.String,
		Picture:     user.Picture.String,
		Website:     user.Website.String,
		Nip05:       user.Nip05.String,
		Lud16:       user.Lud16.String,
		DisplayName: user.DisplayName.String,
	}

	ev.CreatedAt = nostr.Now()
	ev.Kind = nostr.KindProfileMetadata
	c, err := json.Marshal(profile)
	if err != nil {
		log.Println(err)
		return err
	}
	ev.Content = string(c)
	if err := ev.Sign(wrapper.Cfg.PrivateKey); err != nil {
		return err
	}

	fmt.Println(ev)

	var success atomic.Int64
	wrapper.Do(ctx, db.Relay{Write: true}, func(ctx context.Context, relay *nostr.Relay) bool {
		err := relay.Publish(ctx, ev)
		if err != nil {
			slog.Error(relay.URL, "error", err)
		} else {
			slog.Info("Publish to:", "ralay", relay.URL)
			success.Add(1)
		}
		return true
	})

	if success.Load() == 0 {
		return errors.New("cannot send profile metadata")
	}

	return nil
}

func (wrapper *Wrapper) UpdateRelays(relays []db.Relay) {
	wrapper.Cfg.Relays = make(map[string]db.Relay, 0)

	for _, relay := range relays {
		wrapper.Cfg.Relays[relay.Url] = db.Relay{Read: relay.Read, Write: relay.Write, Search: relay.Search}
	}
}
