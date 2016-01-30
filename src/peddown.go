package main

/* Heavily based off of https://github.com/dghubble/go-twitter/blob/master/examples/streaming.go */

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

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
	flags.Parse(os.Args[1:])
	flagutil.SetFlagsFromEnv(flags, "TWITTER")

	if *consumerKey == "" || *consumerSecret == "" || *accessToken == "" || *accessSecret == "" {
		log.Fatal("Consumer key/secret and Access token/secret required")
	}

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
		fmt.Println("Pedestrian Down!")
		fmt.Println(tweet.Text)
		fmt.Printf("https://twitter.com/%s/status/%s\n", tweet.User.ScreenName, tweet.IDStr)

		//If not @yyctransport skip
		if tweet.User.ID != 1729579022 {
			return
		}

		// Determine suffix from number
		number := 0
		body, err := ioutil.ReadFile("counter.txt")
		numberString := string(body)
		if err != nil {
			fmt.Println("counter file does not exist")
		} else {
			number, err = strconv.Atoi(numberString)
			if err != nil {
				fmt.Println("not a valid string")
				number = 1
			}
		}
		number += 1

		//Convert number to byte array and save the number
		numberString = strconv.Itoa(number)
		body = []byte(numberString)
		ioutil.WriteFile("counter.txt", body, 0644)

		suffix := "th"
		switch number % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}

		tweetContent := fmt.Sprintf("#PedestrianDown #yycwalk %d%s tweeted incident in 2016.\n\nhttps://twitter.com/yyctransport/status/%s\n", number, suffix, tweet.IDStr)
		tweet, resp, err := client.Statuses.Update(tweetContent, nil)
		fmt.Println(resp)
		fmt.Println(err)
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
