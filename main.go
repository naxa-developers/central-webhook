// Wrapper for the main tool functionality

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hotosm/central-webhook/db"
	"github.com/hotosm/central-webhook/parser"
	"github.com/hotosm/central-webhook/webhook"
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

func SetupWebhook(
	log *slog.Logger,
	ctx context.Context,
	dbPool *pgxpool.Pool,
	updateEntityUrl, newSubmissionUrl, reviewSubmissionUrl string,
) error {
	// setup the listener
	listener := db.NewListener(dbPool)
	if err := listener.Connect(ctx); err != nil {
		log.Error("error setting up listener: %v", "error", err)
		return err
	}

	// init the trigger function
	db.CreateTrigger(ctx, dbPool, "audits")

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
				eventData := string(data)
				log.Debug("got notification", "data", eventData)

				parsedData, err := parser.ParseEventJson(log, ctx, []byte(eventData))
				if err != nil {
					log.Error("failed to parse notification", "error", err)
					continue // Skip processing this notification
				}

				// Only send the request for correctly parsed (supported) events
				if parsedData != nil {
					if parsedData.Type == "entity.update.version" && updateEntityUrl != "" {
						webhook.SendRequest(log, ctx, updateEntityUrl, *parsedData)
					} else if parsedData.Type == "submission.create" && newSubmissionUrl != "" {
						webhook.SendRequest(log, ctx, newSubmissionUrl, *parsedData)
					} else if parsedData.Type == "submission.update" && reviewSubmissionUrl != "" {
						webhook.SendRequest(log, ctx, reviewSubmissionUrl, *parsedData)
					} else {
						log.Warn("unknown event type or no webhook URL provided", "eventType", parsedData.Type)
					}
				}
			}
		}
	}()

	// unsubscribe after 60s
	// go func() {
	// 	time.Sleep(3 * time.Second)
	// 	sub.Unlisten(ctx)
	// }()

	stopCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Listen for termination signals (e.g., SIGINT/SIGTERM)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		log.Info("received shutdown signal")
		cancel()
	}()

	<-stopCtx.Done()
	log.Info("application shutting down")

	return nil
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

	// Read environment variables
	defaultDbUri := os.Getenv("CENTRAL_WEBHOOK_DB_URI")
	defaultUpdateEntityUrl := os.Getenv("CENTRAL_WEBHOOK_UPDATE_ENTITY_URL")
	defaultNewSubmissionUrl := os.Getenv("CENTRAL_WEBHOOK_NEW_SUBMISSION_URL")
	defaultReviewSubmissionUrl := os.Getenv("CENTRAL_WEBHOOK_REVIEW_SUBMISSION_URL")
	defaultLogLevel := os.Getenv("CENTRAL_WEBHOOK_LOG_LEVEL")

	var dbUri string
	flag.StringVar(&dbUri, "db", defaultDbUri, "DB host (postgresql://{user}:{password}@{hostname}/{db}?sslmode=disable)")

	var updateEntityUrl string
	flag.StringVar(&updateEntityUrl, "updateEntityUrl", defaultUpdateEntityUrl, "Webhook URL for update entity events")

	var newSubmissionUrl string
	flag.StringVar(&newSubmissionUrl, "newSubmissionUrl", defaultNewSubmissionUrl, "Webhook URL for new submission events")

	var reviewSubmissionUrl string
	flag.StringVar(&reviewSubmissionUrl, "reviewSubmissionUrl", defaultReviewSubmissionUrl, "Webhook URL for review submission events")

	var debug bool
	flag.BoolVar(&debug, "debug", false, "Enable debug logging")

	flag.Parse()

	// Set logging level
	var logLevel slog.Level
	if debug {
		logLevel = slog.LevelDebug
	} else if strings.ToLower(defaultLogLevel) == "debug" {
		logLevel = slog.LevelDebug
	} else {
		logLevel = slog.LevelInfo
	}
	log := getDefaultLogger(logLevel)

	if dbUri == "" {
		fmt.Fprintf(os.Stderr, "DB URI is required\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if updateEntityUrl == "" && newSubmissionUrl == "" && reviewSubmissionUrl == "" {
		fmt.Fprintf(os.Stderr, "At least one of updateEntityUrl, newSubmissionUrl, reviewSubmissionUrl is required\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Get a connection pool
	dbPool, err := db.InitPool(ctx, log, dbUri)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not connect to database: %v", err)
		os.Exit(1)
	}

	printStartupMsg()
	err = SetupWebhook(log, ctx, dbPool, updateEntityUrl, newSubmissionUrl, reviewSubmissionUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error setting up webhook: %v", err)
		os.Exit(1)
	}
}
