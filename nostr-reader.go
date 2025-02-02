// Get nostr notes and use api to retrieve them from a postgresql database.
package main

import (
	"amavis442/nostr-reader/internal/config"
	"amavis442/nostr-reader/internal/db"
	"amavis442/nostr-reader/internal/http"
	wrapper "amavis442/nostr-reader/internal/nostr"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

const name = "nostr-reader"

const version = "0.0.10"

func getFireSignalsChannel() chan os.Signal {

	c := make(chan os.Signal, 1)
	signal.Notify(c,
		// https://www.gnu.org/software/libc/manual/html_node/Termination-Signals.html
		syscall.SIGTERM, // "the normal way to politely ask a program to terminate"
		syscall.SIGINT,  // Ctrl+C
		syscall.SIGQUIT, // Ctrl-\
		//syscall.SIGKILL, // "always fatal", "SIGKILL and SIGSTOP may not be caught by a program"
		syscall.SIGHUP, // "terminal is disconnected"
	)
	return c
}

func main() {
	logFile, err := os.OpenFile("log.txt", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
	logger := slog.New(slog.NewJSONHandler(mw, nil))
	slog.NewTextHandler(mw, nil)
	slog.SetDefault(logger)
	defer logFile.Close()

	exitChan := getFireSignalsChannel()
	done := make(chan bool, 1)
	go func() {
		sig := <-exitChan
		logFile.Close()
		fmt.Println(sig)
		done <- true
	}()

	go func() {
		<-done
		fmt.Println("Closing app")
		os.Exit(0)
	}()

	devMode := false
	var cleanStorage bool = false

	helpPtr := flag.Bool("h", false, "Show help dialog")
	modePtr := flag.Bool("dev", false, "Run in dev mode?")
	disableSyncPtr := flag.Bool("disable-sync", false, "Disable sync? can be handy to test swaggerui/api?")
	versionPtr := flag.Bool("version", false, "Show version")
	namePtr := flag.Bool("name", false, "Show exec name")
	syncIntervalPtr := flag.Int("sync", 5, "What is the time (in minutes) between sync of relays to local database?")
	cleanPtr := flag.Bool("clean", false, "Clean database after x days retention?")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] [name ...]\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if *cleanPtr {
		cleanStorage = true
	}

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

	slog.Info("Running in dev mode ", "mode", *modePtr)
	slog.Info("Cleaning database ", "cleanit", *cleanPtr)
	slog.Info("Sync interval is: " + fmt.Sprint(*syncIntervalPtr) + " minutes")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Println(err.Error())
		os.Exit(0)
	}

	cfg.Interval = uint(*syncIntervalPtr)
	if cfg.Interval < 1 {
		slog.Info("Setting interval to 1 minute. This is the minimum")
		cfg.Interval = 1
	}

	slog.Info(fmt.Sprintf("Your public key is: %s", cfg.Nostr.PubKey))
	slog.Info(fmt.Sprintf("Your npub is: %s", cfg.Nostr.Npub))
	slog.Info(fmt.Sprintf("Your nsec is: %s", cfg.Nostr.Nsec))

	var ctx context.Context = context.Background()
	var nostrWrapper wrapper.Wrapper

	nostrWrapper.SetConfig(cfg.Nostr)

	var st db.Storage
	st.SetEnvironment(cfg.Env)
	st.Pubkey = cfg.Nostr.PubKey

	err = st.Connect(ctx, cfg.Database) // Does not make a connection immediately but prepares so it does not yet know if the pg server is available.

	if err != nil {
		log.Println(err.Error())
		os.Exit(0)
	}

	if cleanStorage {
		slog.Info(fmt.Sprintf("Cleaning database entries older then %d days.", st.DbConfig.Retention))
		_ = st.Clean(ctx)
	}
	relays := st.GetRelays(ctx)
	nostrWrapper.UpdateRelays(relays)

	var wg sync.WaitGroup

	intervalTimer := time.Duration(*syncIntervalPtr * 60)
	ticker := time.NewTicker(intervalTimer * time.Second)

	// Creating channel using make
	tickerChan := make(chan bool)

	go func() {
		for {
			select {
			case <-tickerChan:
				return
			// interval task
			case tm := <-ticker.C:
				slog.Info("The Current time is", "time", tm)
				wg.Add(1)
				go intervalTask(&wg, ctx, &st, &nostrWrapper, 120, *disableSyncPtr)
			}
		}
	}()

	var httpServer http.HttpServer
	httpServer.DevMode = devMode
	httpServer.Server = cfg.Server
	httpServer.Database = &st
	httpServer.Nostr = &nostrWrapper

	httpServer.Start()

	wg.Wait()
}

func intervalTask(wg *sync.WaitGroup, ctx context.Context, st *db.Storage, nostrWrapper *wrapper.Wrapper, timeOut int, syncDisabled bool) {
	if syncDisabled {
		return
	}

	tOut := time.Duration(timeOut) * time.Second
	ctx, cancel := context.WithTimeout(ctx, tOut)
	defer func() {
		wg.Done()
		cancel()
	}()

	createdAt := st.GetLastTimeStamp(ctx)
	t := time.Unix(createdAt, 0)
	slog.Info(fmt.Sprint("TimeStamps: ", createdAt, t.UTC()))
	filter := nostrWrapper.GetEventData(createdAt, false)
	evs := nostrWrapper.GetEvents(ctx, filter)

	_, err := st.SaveEvents(ctx, evs)
	if err != nil {
		slog.Error(err.Error())
	}

	if len(db.Missing_event_ids) > 0 {
		slog.Info("Sniping missing events...........")
		//need to try to get them
		filter = nostr.Filter{
			IDs: db.Missing_event_ids,
		}
		evs := nostrWrapper.GetEvents(ctx, filter)

		_, err := st.SaveEvents(ctx, evs)
		if err != nil {
			log.Println(err)
		}
	}

	slog.Info("Done syncing")

}
