# Pedestrian Down ([@PedDownYYC](https://twitter.com/PedDownYYC))

A simple Twitter bot that will quote tweet any @yyctransport tweet with the words:
"ALERT" and "pedestrian" or "ped" but not cleared. In the tweet it mentions the number and
tags #yycwalk. It's by no means scientific or accurate as it assumes @yyctransport
is tweeting every collision (see Caveats) and if a tweet doesn't include ped or pedestrian
it could be missed.

Thanks to @yyctransport for reporting the incidents, and [@RrrichardZach](https://twitter.com/RrrichardZach/status/690322441403367424)
for the push to create the bot.

## Caveats

Since the bot is dependant on the @yyctransport team at the [Traffic Management Centre](http://calgary.ca/Transportation/Roads/Pages/Traffic/Traffic-management/Traffic-management.aspx)
tweeting, not all incidents are counted and reported. For the most accurate stats, please
check with @CalgaryPolice. Incidents if viewed via camera, reported via 911, and affect
traffic are tweeted. [[1]](https://twitter.com/yyctransport/status/697156806250930176)[[2]](https://twitter.com/yyctransport/status/697156999507644416)

Even with this limitation, the purpose to show just how frequent it happens (and impacts
traffic) is a worthwhile investigation.

----

1. Requires Go (only tested with 1.5.3)
2. Twitter Credentials are environment variables in the Upstart file.

If you don't already have a Go workspace set up, use the peddown directory.
Also - this is likely not best practice. I'm rather new at Go.

    export GOPATH=/home/ubuntu/peddown

Install dependencies:

    go get github.com/dghubble/go-twitter/twitter
    go get github.com/coreos/pkg/flagutil
    go get github.com/dghubble/oauth1

Then:

    go build src/peddown.go

Logs if using the Upstart file go to `/var/log/upstart/peddown.log`

