// Wrapper for the tool functionality

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hotosm/odk-webhook/db"
	"github.com/hotosm/odk-webhook/webhook"
)

func getDefaultLogger(lvl slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     lvl,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				source, _ := a.Value.Any().(*slog.Source)
				if source != nil {
					source.Function = ""
					source.File = filepath.Base(source.File)
				}
			}
			return a
		},
	}))
}

// triggerableActions options:
//   - entity.update.version  (entity edit)
//   - submission.create  (submission creation)
//
// See ODK docs for all options
func SetupWebhook(
	log *slog.Logger,
	ctx context.Context,
	dbPool *pgxpool.Pool,
	webhookUrl string,
	triggerableActions map[string]bool,
) error {
	// setup the listener
	listener := db.NewListener(dbPool)
	if err := listener.Connect(ctx); err != nil {
		log.Error("error setting up listener: %v", "error", err)
		return err
	}

	// init the trigger function
	db.CreateTrigger(ctx, dbPool, "odk-events")

	// setup the notifier
	notifier := db.NewNotifier(log, listener)
	go notifier.Run(ctx)

	// subscribe to the 'odk-events' channel
	log.Info("listening to odk-events channel")
	sub := notifier.Listen("odk-events")

	// indefinitely listen for updates
	go func() {
		<-sub.EstablishedC()
		for {
			select {

			case <-ctx.Done():
				sub.Unlisten(ctx)
				log.Info("done listening for notifications")
				return

			case data := <-sub.NotificationC():
				dataString := string(data)
				log.Debug("got notification: %s \n", "data", dataString)

				parsedData, err := webhook.ParseEventJson(log, ctx, []byte(data))
				if err != nil {
					log.Error("Failed to parse notification", "error", err)
					continue // Skip processing this notification
				}

				if triggerableActions[parsedData.Action] {
					webhook.SendRequest(log, ctx, webhookUrl, *parsedData)
				} else {
					log.Debug("Event type is not set to trigger webhook", "type", parsedData.Action)
				}
			}
		}
	}()

	// unsubscribe after 60s
	// go func() {
	// 	time.Sleep(3 * time.Second)
	// 	sub.Unlisten(ctx)
	// }()

	select {}
}

func parseTriggerFlag(trigger string) (map[string]bool, error) {
	trigger = strings.ToLower(strings.TrimSpace(trigger))
	triggerableActions := make(map[string]bool)

	switch trigger {
	// case "all":
	// 	triggerableActions["entity.update.version"] = true
	// 	triggerableActions["submission.create"] = true
	// 	// TODO add more options here
	case "entities":
		triggerableActions["entity.update.version"] = true
	case "submissions":
		triggerableActions["submission.create"] = true
	case "submissions,entities", "entities,submissions":
		triggerableActions["entity.update.version"] = true
		triggerableActions["submission.create"] = true
	default:
		return nil, fmt.Errorf("invalid trigger value: %s", trigger)
	}

	return triggerableActions, nil
}

func printStartupMsg() {
	banner := `
   ____  _____  _  __ __          __  _     _                 _    
  / __ \|  __ \| |/ / \ \        / / | |   | |               | |   
 | |  | | |  | | ' /   \ \  /\  / /__| |__ | |__   ___   ___ | | __
 | |  | | |  | |  <     \ \/  \/ / _ \ '_ \| '_ \ / _ \ / _ \| |/ /
 | |__| | |__| | . \     \  /\  /  __/ |_) | | | | (_) | (_) |   < 
  \____/|_____/|_|\_\     \/  \/ \___|_.__/|_| |_|\___/ \___/|_|\_\                                                          
	`
	fmt.Println(banner)
	fmt.Println("")
}

func main() {
	ctx := context.Background()
	log := getDefaultLogger(slog.LevelInfo)

	var dbUri string
	flag.StringVar(&dbUri, "db", "", "DB host (postgresql://{user}:{password}@{hostname}/{db}?sslmode=disable)")

	var webhookUri string
	flag.StringVar(&webhookUri, "webhook", "", "Webhook URL to call")

	var trigger string
	flag.StringVar(&trigger, "trigger", "submissions,entities", "Trigger actions (submissions, entities, or 'submissions,entities' for both)")

	flag.Parse()

	if dbUri == "" || webhookUri == "" {
		fmt.Fprintf(os.Stderr, "missing required flags\n")
		flag.PrintDefaults()
		os.Exit(1)
		return
	}

	triggerableActions, err := parseTriggerFlag(trigger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing trigger flag: %v\n", err)
		os.Exit(1)
	}

	// get a connection pool
	dbPool, err := db.InitPool(ctx, log, dbUri)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not connect to database: %v", err)
		os.Exit(1)
	}

	printStartupMsg()
	err = SetupWebhook(log, ctx, dbPool, webhookUri, triggerableActions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error setting up webhook: %v", err)
		os.Exit(1)
	}
}
