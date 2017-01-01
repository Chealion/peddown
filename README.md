# Pedestrian Down ([@PedDownYYC](https://twitter.com/PedDownYYC))

A Twitter bot and website collecting and tweeting pedestrian incidents reported
by the @yyctransport Twitter account and OpenData Portal.

Unforunately the tweets and data from the OpenData Portal do not coincide in time
or share any uniquely identifiable information, but using the intersection one can
roughly correlate the data between the two public sources.

## tweetCollector.go

This tool like the original version watches @yyctransport for the words: "ALERT",
"pedestrian" or "ped" and adds it to the database and retweets the tweet.

## peddown.go

TBD

## History

The initial version of this was a Twitter only bot that would quote any tweet from
@yyctransport with the words: "ALERT" and "pedestrian" or "ped" but not cleared. In 
the tweet it mentions the number and tags #yycwalk. 

Thanks to @yyctransport for reporting the incidents, and [@RrrichardZach](https://twitter.com/RrrichardZach/status/690322441403367424)
for the push to create the bot.

## Future Plans

Currently investigating the new Open Data Portal from the City of Calgary.

## Caveats

Since the bot is dependant on the @yyctransport team at the [Traffic Management Centre](http://calgary.ca/Transportation/Roads/Pages/Traffic/Traffic-management/Traffic-management.aspx)
tweeting, not all incidents are counted and reported. For the most accurate stats, please
check with @CalgaryPolice. Incidents if viewed via camera, reported via 911, and affect
traffic are tweeted. [[1]](https://twitter.com/yyctransport/status/697156806250930176)[[2]](https://twitter.com/yyctransport/status/697156999507644416)

Even with this limitation, the purpose to show just how frequent incidents occur.

----

1. Requires Go
2. Twitter Credentials are environment variables in the Upstart file.

If you don't already have a Go workspace set up, use the peddown directory.
Also - this is likely not best practice. I'm rather new at Go.

    export GOPATH=/home/ubuntu/peddown

Install dependencies:

    go get -u github.com/dghubble/go-twitter/twitter
    go get -u github.com/coreos/pkg/flagutil
    go get -u github.com/dghubble/oauth1
    go get -u github.com/mattn/go-sqlite3

Then:

    go build src/go-peddown2/tweetCollector/tweetCollector.go
    mv tweetCollector peddown

Logs if using the Upstart file go to `/var/log/upstart/peddown.log`

## LICENSE

Licensed with an [MIT License](http://choosealicense.com/licenses/mit/)
