# Pedestrian Down

With increasing deaths on Calgary's streets, it's over due for the bot to come back and remind folks of how dangerous our streets are.

## History

The initial version of this was a Twitter only bot that would quote any tweet from
@yyctransport with the words: "ALERT" and "pedestrian" or "ped" but not cleared. In
the tweet it mentions the number and tags #yycwalk.

When Twitter/X killed bots by destroying their API, the bot was shut down.

Thanks to @yyctransport for reporting the incidents, and [@RrrichardZach](https://twitter.com/RrrichardZach/status/690322441403367424)
for the push to create the bot.

## Caveats

Since the bot is dependant on the @yyctransport team at the [Traffic Management Centre](http://calgary.ca/Transportation/Roads/Pages/Traffic/Traffic-management/Traffic-management.aspx)
tweeting, not all incidents are counted and reported. For the most accurate stats, please
check with @CalgaryPolice. Incidents if viewed via camera, reported via 911, and affect
traffic are tweeted. [[1]](https://twitter.com/yyctransport/status/697156806250930176)[[2]](https://twitter.com/yyctransport/status/697156999507644416)

Even with this limitation, the purpose to show just how frequent incidents occur.

## Datasets

* [Incidents (35ra-9556)](https://data.calgary.ca/Transportation-Transit/Traffic-Incidents/35ra-9556/)
* [Ward Boundaries (4b54-tmc4)](https://data.calgary.ca/Government/Ward-Boundaries/4b54-tmc4/)
* [Communities (kxmf-bzkv)](https://data.calgary.ca/Government/Calgary-Communities/kxmf-bzkv/)
* Councillors - manually created into a json config blob.

## Technology Stack

* Language: Go
* Database: Sqlite3

After trying [go-soda](https://github.com/SebastiaanKlippert/go-soda), I opted to move to using [earthboundkid/requests](https://github.com/earthboundkid/requests) instead so I could use the v3 API which provides extra fields.

## Deployment Details

Please see [docs](./docs)

## LICENSE

Licensed with an [MIT License](http://choosealicense.com/licenses/mit/)
