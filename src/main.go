package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

const name = "nostr-reader"

const version = "0.0.10"

func main() {
	devMode := false

	helpPtr := flag.Bool("h", false, "Show help dialog")
	modePtr := flag.Bool("dev", false, "Run in dev mode?")
	versionPtr := flag.Bool("version", false, "Show version")
	namePtr := flag.Bool("name", false, "Show name")
	syncIntervalPtr := flag.Int("sync", 5, "What is the time (in minutes) between sync of relays to local database?")

	flag.Parse()

	if *modePtr {
		devMode = true
	}
	if *helpPtr {
		flag.Usage()
		return
	}
	if *versionPtr {
		fmt.Println(version)
		return
	}

	if *namePtr {
		fmt.Println(name)
		return
	}

	fmt.Println("Running in dev mode: ", *modePtr)
	fmt.Println("Sync interval is: ", *syncIntervalPtr, " minutes")

	cfg, err := LoadConfig()
	if err != nil {
		log.Println(err.Error())
		os.Exit(0)
	}
	if cfg.Server.Interval < 1 {
		log.Println("Setting interval to 1 minute. This is the minimum")
		cfg.Server.Interval = 1
	}

	fmt.Println("Your public key is: ", cfg.Nostr.PubKey)
	fmt.Println("Your npub is: ", cfg.Nostr.Npub)
	fmt.Println("Your nsec is: ", cfg.Nostr.Nsec)

	var ctx context.Context = context.Background()
	var nostrWrapper Wrapper

	nostrWrapper.SetConfig(cfg.Nostr)

	var st Storage
	st.SetEnvironment(cfg.Env)
	st.Pubkey = cfg.Nostr.PubKey

	err = st.Connect(ctx, cfg.Database) // Does not make a connection immediately but prepares so it does not yet know if the pg server is available.

	var gq GQ
	gq.Config = cfg.Database
	gq.Connect(ctx)

	if err != nil {
		log.Println(err.Error())
		os.Exit(0)
	}

	relays := st.GetRelays(ctx)
	nostrWrapper.UpdateRelays(relays)

	if !(cfg.Server.Interval > 0) {
		log.Println("Please set the interval in minutes in config.json")
		os.Exit(0)
	}

	intervalTimer := time.Duration(*syncIntervalPtr * 60)
	ticker := time.NewTicker(intervalTimer * time.Second)

	var wg sync.WaitGroup
	// Creating channel using make
	tickerChan := make(chan bool)

	go func() {
		for {
			select {
			case <-tickerChan:
				return
			// interval task
			case tm := <-ticker.C:
				log.Println("The Current time is: ", tm)
				wg.Add(1)
				go intervalTask(&wg, ctx, &st, &nostrWrapper, 120)
			}
		}
	}()

	var httpServer HttpServer
	httpServer.DevMode = devMode
	httpServer.Server = cfg.Server
	httpServer.Database = &st
	httpServer.Nostr = &nostrWrapper

	httpServer.Start()

	wg.Wait()

}

func intervalTask(wg *sync.WaitGroup, ctx context.Context, st *Storage, nostrWrapper *Wrapper, timeOut int) {
	tOut := time.Duration(timeOut) * time.Second
	ctx, cancel := context.WithTimeout(ctx, tOut)
	defer func() {
		wg.Done()
		cancel()
	}()

	createdAt := st.GetLastTimeStamp(ctx)
	t := time.Unix(createdAt, 0)
	log.Println("TimeStamps: ", createdAt, t.UTC())
	filter := nostrWrapper.GetEventData(createdAt, false)
	evs := nostrWrapper.GetEvents(ctx, filter)

	_, err := st.SaveEvents(ctx, evs)
	if err != nil {
		log.Println(err)
	}
	log.Println("Done syncing")

}
