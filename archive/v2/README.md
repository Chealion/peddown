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

## Golang

1. Clone this repository
2. Grab the dependencies via `go mod download`
3. `go build .`

## Docker (x86)

    docker build -t peddown .
    docker run peddown

# Deployment Instructions

The credentials for Twitter are read from the environment. If using the included systemd service file, add them there.
If using Docker or something else to run them, please see Docker's instructions on managing environment variables.

## fly.io

1. Create the app using `flyctl launch` (Do NOT choose to deploy)
2. Load the four environment variables in creds.example using `flyctl secrets`

    sed 's/export //g' creds | flyctl secrets import

3. Create the volume if necessary with `flyctl volume create peddown_data --region sea --size 1`
3. Optionally restore the database with:

   flyctl ssh sftp shell
   cd /data
   put peddown.db peddown.sqlite

4. Deploy the applicaton with `flyctl deploy`
5. Monitor with `flyctl logs`

### Backups

Run a cron elsewhere to grab the file :( Maybe I'll investigate [LiteFS](https://fly.io/blog/introducing-litefs/) in the future but that's a lot more work. Instead we'll set up a cron locally on my laptop to:

    flyctl ssh sftp get /peddown.db peddown.db

## Manual Runs and Pushing Updates

    cd ~/src/peddown.git
    flyctl ssh sftp get /peddown.db peddown.db
    source creds
    ~/src/peddown.git % go run tweetCollector.go -manual https://twitter.com/yyctransport/status/<ID>
    flyctl ssh console -C 'mv /data/peddown.sqlite /data/peddown.sqlite.bk'
    echo 'put peddown.db /data/peddown.sqlite' | flyctl ssh sftp shell

## Updating dependencies
Run:
    go get -u && go mod tidy

## LICENSE

Licensed with an [MIT License](http://choosealicense.com/licenses/mit/)
