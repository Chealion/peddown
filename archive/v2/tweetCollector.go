package main

/* Heavily based off of https://github.com/dghubble/go-twitter/blob/master/examples/streaming.go */

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chealion/peddown/database"

	"github.com/coreos/pkg/flagutil"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
)

// Global Variables
var databasePath string
var hashtags string
var ministers string

func processTweet(tweet *twitter.Tweet, twitterClient *twitter.Client, databaseConn *sql.DB) {
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

	var err error

	// Some time math for interacting with SQLite from the Ruby Date in the Tweet object
	tweetTime, err := time.Parse(time.RubyDate, tweet.CreatedAt)
	if err != nil {
		log.Fatal("Error parsing tweet time")
	}
	tweetDate := tweetTime.Format("2006-01-02")
	tweetYear := tweetTime.Format("2006")

	// Determine number of tweet in the year
	// -7 hours to cover the 7 hours on Dec. 31 when Calgary is not in the same year as UTC
	query := fmt.Sprintf("select count(*)+1 from tweets where strftime('%%Y', tweetDate, '-7 hours') = strftime('%%Y', '%s', '-7 hours');", tweetDate)
	err = databaseConn.QueryRow(query).Scan(&number)
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

	tweetContent := fmt.Sprintf("%d%s tweeted pedestrian incident in %s.\n%s\n%s\n\n%s\n", number, suffix, tweetYear, hashtags, ministers, tweetURL)
	tweet, resp, err := twitterClient.Statuses.Update(tweetContent, nil)

	// Log Twitter's response
	fmt.Println(tweetContent)
	fmt.Println(resp)
	fmt.Println(err)

	sqlStmt := `
		insert into tweets(tweetDate, tweetID, tweetURL, tweetText) values (?, ?, ?, ?)
	`
	// Need to convert tweet.CreatedAt to YYYY-MM-DD HH:MM:SS for storage in SQLite
	databaseConn.Exec(sqlStmt, tweetTime.Format("2006-01-02 03:04:05"), tweet.IDStr, tweetURL, tweetContent)
	if err != nil {
		log.Panic(err)
	}
}

/* https://stackoverflow.com/a/44222606 */
func trimQuotes(s string) string {
	if len(s) > 0 && s[0] == '"' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '"' {
		s = s[:len(s)-1]
	}
	return s
}

func main() {

	flags := flag.NewFlagSet("user-auth", flag.ExitOnError)
	consumerKey := flags.String("consumer-key", "", "Twitter Consumer Key")
	consumerSecret := flags.String("consumer-secret", "", "Twitter Consumer Secret")
	accessToken := flags.String("access-token", "", "Twitter Access Token")
	accessSecret := flags.String("access-secret", "", "Twitter Access Secret")

	manualTweet := flags.String("manual", "", "Manually enter a tweet to process")

	databasePath := flags.String("database", "./peddown.db", "Database Path")
	fhashtags := flags.String("hashtags", "", "Hashtags to include")
	fministers := flags.String("ministers", "", "Transportation and Health Critics to include")

	flags.Parse(os.Args[1:])
	flagutil.SetFlagsFromEnv(flags, "TWITTER")
	flagutil.SetFlagsFromEnv(flags, "PEDDOWN")

	// Load globals

	hashtags = trimQuotes(*fhashtags)
	ministers = trimQuotes(*fministers)

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

	// Skip streaming if we're in manual mode
	// ./peddown -manual <URL>
	if *manualTweet != "" {
		// Grab just the last part of the URL
		tweetID, err := strconv.ParseInt(strings.Split(*manualTweet, "/")[len(strings.Split(*manualTweet, "/"))-1], 10, 64)
		if err != nil {
			panic(err)
		}
		tweet, resp, err := client.Statuses.Show(tweetID, nil)

		// Log Twitter's response
		fmt.Println(resp)

		processTweet(tweet, client, database.Conn)

		os.Exit(0)
	}

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

		processTweet(tweet, client, database.Conn)
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
