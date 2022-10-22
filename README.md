# Pedestrian Down ([@PedDownYYC](https://twitter.com/PedDownYYC))

A Twitter bot and website collecting and tweeting pedestrian incidents reported
by the @yyctransport Twitter account and OpenData Portal.

Unforunately the tweets and data from the OpenData Portal do not coincide in time
or share any uniquely identifiable information, but using the intersection one can
roughly correlate the data between the two public sources.

## History

The initial version of this was a Twitter only bot that would quote any tweet from
@yyctransport with the words: "ALERT" and "pedestrian" or "ped" but not cleared. In
the tweet it mentions the number and tags #yycwalk.

Thanks to @yyctransport for reporting the incidents, and [@RrrichardZach](https://twitter.com/RrrichardZach/status/690322441403367424)
for the push to create the bot.

## Caveats

Since the bot is dependant on the @yyctransport team at the [Traffic Management Centre](http://calgary.ca/Transportation/Roads/Pages/Traffic/Traffic-management/Traffic-management.aspx)
tweeting, not all incidents are counted and reported. For the most accurate stats, please
check with @CalgaryPolice. Incidents if viewed via camera, reported via 911, and affect
traffic are tweeted. [[1]](https://twitter.com/yyctransport/status/697156806250930176)[[2]](https://twitter.com/yyctransport/status/697156999507644416)

Even with this limitation, the purpose to show just how frequent incidents occur.

----
# Build Instructions

1. Clone this repository
2. Grab the dependencies via `go mod download`
3. `go build .`

# Deployment Instructions

The credentials for Twitter are read from the environment. If using the included systemd service file, add them there.
If using Docker or something else to run them, please see Docker's instructions on managing environment variables.

## Updating dependencies
Run:
    go get -u && go mod tidy

## LICENSE

Licensed with an [MIT License](http://choosealicense.com/licenses/mit/)
