# Pedestrian Down

A simple Twitter bot that will quote tweet any @yyctransport tweet with the words:
"ALERT" and "pedestrian" or "ped" but not cleared. In the tweet it mentions the number and
tags #yycwalk. It's by no means scientific or accurate as it assumes @yyctransport
is tweeting every collision and if a tweet doesn't include ped or pedestrian it could be
mised.

For me, it's just a chance to force myself to learn more Go.

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

Logs if using the Upstart file go to /var/log/upstart/peddown.log

