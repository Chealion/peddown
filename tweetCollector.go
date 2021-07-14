package main

/* Heavily based off of https://github.com/dghubble/go-twitter/blob/master/examples/streaming.go */

import "go-peddown2/database"
import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coreos/pkg/flagutil"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
)

func main() {

	flags := flag.NewFlagSet("user-auth", flag.ExitOnError)
	consumerKey := flags.String("consumer-key", "", "Twitter Consumer Key")
	consumerSecret := flags.String("consumer-secret", "", "Twitter Consumer Secret")
	accessToken := flags.String("access-token", "", "Twitter Access Token")
	accessSecret := flags.String("access-secret", "", "Twitter Access Secret")
	databasePath := flags.String("database", "./peddown.db", "Database Path")
	flags.Parse(os.Args[1:])
	flagutil.SetFlagsFromEnv(flags, "TWITTER")

	if *consumerKey == "" || *consumerSecret == "" || *accessToken == "" || *accessSecret == "" {
		log.Fatal("Consumer key/secret and Access token/secret required")
	}

	var err error
	database.Conn, err = sql.Open("sqlite3", *databasePath)
	if err != nil {
		log.Fatal(err)
	}

	// Confirm database is populated
	database.CheckDatabase(database.Conn)
	defer database.Conn.Close()

	config := oauth1.NewConfig(*consumerKey, *consumerSecret)
	token := oauth1.NewToken(*accessToken, *accessSecret)
	// OAuth1 http.Client will automatically authorize Requests
	httpClient := config.Client(oauth1.NoContext, token)

	// Twitter Client
	client := twitter.NewClient(httpClient)

	// Convenience Demux demultiplexed stream messages
	// What to do with each type of tweet
	demux := twitter.NewSwitchDemux()

	demux.Tweet = func(tweet *twitter.Tweet) {
		// Log that something was found.
		fmt.Println("Pedestrian Down!")
		fmt.Println(tweet.FullText)
		fmt.Printf("https://twitter.com/%s/status/%s\n", tweet.User.ScreenName, tweet.IDStr)

		//If not @yyctransport skip
		if tweet.User.ID != 1729579022 {
			return
		}

		// Save information to the database here.
		/*
		*	id (integer)
		* incidentid (int) = -1
		* tweetDate <tweet.created_at)
		* tweetID tweet.IDStr
		* tweetURL fmt.Printf("https://twitter.com/%s/status/%s\n", tweet.User.ScreenName, tweet.IDStr)
		* tweetText tweet.FullText
		 */

		tweetURL := fmt.Sprintf("https://twitter.com/yyctransport/status/%s", tweet.IDStr)
		number := 0

		err = database.Conn.QueryRow("select count(*)+1 from tweets where strftime('%Y', tweetDate) = strftime('%Y', 'now');").Scan(&number)
		if err != nil {
			log.Fatal("Database error", err)
		}

		suffix := "th"
		switch number % 10 {
		case 1:
			if (number % 100) != 11 {
				suffix = "st"
			}
		case 2:
			if (number % 100) != 12 {
				suffix = "nd"
			}
		case 3:
			if (number % 100) != 13 {
				suffix = "rd"
			}
		}

		t := time.Now()
		year := t.Format("2006")

		tweetContent := fmt.Sprintf("%d%s tweeted pedestrian incident in %s.\n#yycwalk #yyccc #ableg #visionzero\n@RajanJSaw @shandro\n@lornedach @DShepYEG\n\n%s\n", number, suffix, year, tweetURL)
		tweet, resp, err := client.Statuses.Update(tweetContent, nil)

		// Log Twitter's response
		fmt.Println(tweetContent)
		fmt.Println(resp)
		fmt.Println(err)

		sqlStmt := `
			insert into tweets(tweetDate, tweetID, tweetURL, tweetText) values (?, ?, ?, ?)
		`
		// Need to convert tweet.CreatedAt to YYYY-MM-DD HH:MM:SS for storage in SQLite
		tweetTime, err := time.Parse(time.RubyDate, tweet.CreatedAt)
		if err != nil {
			log.Panic(err)
		}
		database.Conn.Exec(sqlStmt, tweetTime.Format("2006-01-02 03:04:05"), tweet.IDStr, tweetURL, tweetContent)
		if err != nil {
			log.Panic(err)
		}
	}

	fmt.Println("Starting Stream...")

	// FILTER
	filterParams := &twitter.StreamFilterParams{
		Track:         []string{"ALERT ped,ALERT pedestrian"},
		StallWarnings: twitter.Bool(true),
	}
	stream, err := client.Streams.Filter(filterParams)
	if err != nil {
		log.Fatal(err)
	}

	// Receive messages until stopped or stream quits
	go demux.HandleChan(stream.Messages)

	// Wait for SIGINT and SIGTERM (HIT CTRL-C)
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.Println(<-ch)

	fmt.Println("Stopping Stream...")
	stream.Stop()
}
