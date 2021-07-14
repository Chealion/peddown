package database

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

const (
	currentVersion = 1
)

var (
	Conn *sql.DB
)

// Check Datbase. If emtpy, create tables
func CheckDatabase(db *sql.DB) {

	rows, err := db.Query("select version from metadata")

	// If no such table create tables
	if err != nil && err.Error() == "no such table: metadata" {

		// Metadata table
		sqlStmt := `
	create table metadata (version integer not null);
	`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			log.Printf("%q: %s\n", err, sqlStmt)
			return
		}
		_, err = db.Exec("insert into metadata(version) values(1)")
		if err != nil {
			log.Fatal(err)
		}

		// Tweet Table
		sqlStmt = `
		create table if not exists tweets (
			id integer primary key,
			incidentId	int	default -1,
			tweetDate datetime,
			tweetId str,
			tweetURL str,
			tweetText str
			retweeted int default 0,
			retweetId str
		);
		`

		_, err = db.Exec(sqlStmt)
		if err != nil {
			log.Printf("%q: %s\n", err, sqlStmt)
			return
		}

		// Incident Table
		sqlStmt = `
		create table if not exists traffic_incidents (
			id integer primary key,
			incidentDate datetime,
			description string,
			quardrant string,
			longtitude string,
			latitude string,
			location string,
			pedestrian int,
			ourTweetId int default -1
	);
	`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			log.Printf("%q: %s\n", err, sqlStmt)
			return
		}

	} else if err != nil {
		//Error not handled. Bail out!
		log.Fatal(err)

	} else {

		// Query suceeded. Do we need to migrate schemas?
		for rows.Next() {
			var version int
			err = rows.Scan(&version)

			if err != nil {
				log.Fatal(err)
			}
			if currentVersion > version {
				fmt.Println("Old Version of DB installed. Migration code could go here")
			}
		}
		rows.Close()
	}

}

func main() {
	//Open the database. Will create a new one if it doesn't exist.
	/*	db, err := sql.Open("sqlite3", "./peddown.db")
		if err != nil {
			log.Fatal(err)
		}

		// Check if database is populated
		checkDatabase(db)

		defer db.Close() */
}
