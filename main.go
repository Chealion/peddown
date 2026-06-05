package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// mountain is the timezone used for all user-facing dates/times in posts and
// for the per-calendar-year row numbering. Pinned explicitly so behavior does
// not depend on the host TZ (fly.io VMs run in UTC).
var mountain *time.Location

func init() {
	loc, err := time.LoadLocation("America/Edmonton")
	if err != nil {
		log.Fatalf("load America/Edmonton tz (is tzdata installed?): %v", err)
	}
	mountain = loc
}

func main() {
	// Initialize database
	db, err := NewIncidentDB("incidents.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create schema
	if err := db.CreateSchema(); err != nil {
		log.Fatalf("Failed to create schema: %v", err)
	}

	// Development/debug - import using the json file
	// Download the file from https://data.calgary.ca/resource/35ra-9556.json
	if len(os.Args) > 1 && os.Args[1] == "load-local-json" {
		if err := importLocalIncidents(db, "35ra-9556.json"); err != nil {
			log.Fatalf("Failed to import incidents: %v", err)
		}
		fmt.Println("Successfully imported incidents from local json file")
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "load-support-data" {
		if err := importSupportData(db); err != nil {
			log.Fatalf("Failed to import the support data: %v", err)
		}
		fmt.Println("Successfully imported support data")
		return
	}

	// Fetch incidents from data.calgary.ca
	if len(os.Args) > 1 && os.Args[1] == "load" {
		if err := importIncidents(db); err != nil {
			log.Fatalf("Failed to fetch incidents: %v", err)
		}
		fmt.Println("Successfully fetched incidents")
		return
	}

	// Process unprocessed incidents
	if len(os.Args) > 1 && os.Args[1] == "process" {
		if err := processIncidents(db); err != nil {
			log.Fatalf("Failed to process incidents: %v", err)
		}
		return
	}

	fmt.Println("Usage:")
	fmt.Println("  peddown load                - Import pedestrian incidents from data.calgary.ca")
	fmt.Println("  peddown load-support-data   - Import support data from data.calgary.ca")
	fmt.Println("  peddown load-local-json     - Import incidents from 35ra-9556.json")
	fmt.Println("  peddown process             - Process unprocessed pedestrian incidents")
}

func importLocalIncidents(db *IncidentDB, filename string) error {
	// Initialize geo lookup for ward/community calculations
	fmt.Println("Loading geographic data for lookups...")
	geoLookup := NewGeoLookup()
	if err := geoLookup.LoadWards(db); err != nil {
		return fmt.Errorf("load wards for geo lookup: %w", err)
	}
	if err := geoLookup.LoadCommunities(db); err != nil {
		return fmt.Errorf("load communities for geo lookup: %w", err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read file %s: %w", filename, err)
	}

	var incidents []Incident
	if err := json.Unmarshal(data, &incidents); err != nil {
		return fmt.Errorf("unmarshal JSON: %w", err)
	}

	fmt.Printf("Importing %d incidents...\n", len(incidents))

	for i := range incidents {
		// Calculate ward and community from lat/lon
		incidents[i].WardID = geoLookup.FindWard(incidents[i].Latitude, incidents[i].Longitude)
		incidents[i].CommunityID = geoLookup.FindCommunity(incidents[i].Latitude, incidents[i].Longitude)

		if err := db.UpsertIncident(&incidents[i]); err != nil {
			return fmt.Errorf("upsert incident %d: %w", i, err)
		}

		if (i+1)%100 == 0 {
			fmt.Printf("Processed %d incidents...\n", i+1)
		}
	}

	return nil
}

// Import incidents from data.calgary.ca
func importIncidents(db *IncidentDB) error {
	client, err := NewSocrataClient()
	if err != nil {
		return fmt.Errorf("create socrata client: %w", err)
	}

	// Initialize geo lookup for ward/community calculations
	fmt.Println("Loading geographic data for lookups...")
	geoLookup := NewGeoLookup()
	if err := geoLookup.LoadWards(db); err != nil {
		return fmt.Errorf("load wards for geo lookup: %w", err)
	}
	if err := geoLookup.LoadCommunities(db); err != nil {
		return fmt.Errorf("load communities for geo lookup: %w", err)
	}

	fmt.Println("Fetching incidents from data.calgary.ca...")

	// Fetch last 20 incidents
	// Set as variable for changing for massive imports
	incidentCount := 20
	incidents, err := client.FetchIncidents(incidentCount)
	if err != nil {
		return fmt.Errorf("fetch incidents: %w", err)
	}

	fmt.Printf("Fetched %d incidents, importing to database...\n", len(incidents))

	// Import to database
	for i := range incidents {
		// Calculate ward and community from lat/lon
		incidents[i].WardID = geoLookup.FindWard(incidents[i].Latitude, incidents[i].Longitude)
		incidents[i].CommunityID = geoLookup.FindCommunity(incidents[i].Latitude, incidents[i].Longitude)

		if err := db.UpsertIncident(&incidents[i]); err != nil {
			return fmt.Errorf("upsert incident %d: %w", i, err)
		}

		if (i+1)%100 == 0 {
			fmt.Printf("Imported %d incidents...\n", i+1)
		}
	}

	// Report how many new incidents were added
	incidentCount, err = db.GetUnprocessedIncidentCount()
	if err != nil {
		return fmt.Errorf("failed to get unprocessed count: %w", err)
	} else {
		if incidentCount > 0 {
			fmt.Println("")
			fmt.Println("------")
			fmt.Printf("There are %d incidents to process\n", incidentCount)
			fmt.Println("------")
			fmt.Println("")
		} else {
			fmt.Println("No new incidents to report")
		}
	}

	return nil
}

func importSupportData(db *IncidentDB) error {
	// Create Socrata client
	client, err := NewSocrataClient()
	if err != nil {
		return fmt.Errorf("create socrata client: %w", err)
	}

	// Fetch wards
	fmt.Println("Fetching wards from data.calgary.ca...")
	wards, err := client.FetchWards()
	if err != nil {
		return fmt.Errorf("fetch wards: %w", err)
	}

	fmt.Printf("Importing %d wards to database...\n", len(wards))
	for i, ward := range wards {
		if err := db.UpsertWard(&ward); err != nil {
			return fmt.Errorf("upsert ward %d: %w", i, err)
		}
	}

	// Fetch communities
	fmt.Println("Fetching communities from data.calgary.ca...")
	communities, err := client.FetchCommunities()
	if err != nil {
		return fmt.Errorf("fetch communities: %w", err)
	}

	fmt.Printf("Importing %d communities to database...\n", len(communities))
	for i, community := range communities {
		if err := db.UpsertCommunity(&community); err != nil {
			return fmt.Errorf("upsert community %d: %w", i, err)
		}

		if (i+1)%50 == 0 {
			fmt.Printf("Processed %d communities...\n", i+1)
		}
	}

	// Councillors must still be loaded from local file
	fmt.Println("Loading councillors from local file...")
	data, err := os.ReadFile("councillors.json")
	if err != nil {
		return fmt.Errorf("read councillors.json: %w", err)
	}

	var councillors []Councillor
	if err := json.Unmarshal(data, &councillors); err != nil {
		return fmt.Errorf("unmarshal councillors: %w", err)
	}

	fmt.Printf("Importing %d councillors...\n", len(councillors))
	for i, councillor := range councillors {
		if err := db.UpsertCouncillor(&councillor); err != nil {
			return fmt.Errorf("upsert councillor %d: %w", i, err)
		}

		if (i+1)%10 == 0 {
			fmt.Printf("Processed %d councillors...\n", i+1)
		}
	}

	return nil
}

func processIncidents(db *IncidentDB) error {
	incidents, err := db.GetUnprocessedIncidents()
	if err != nil {
		return fmt.Errorf("get unprocessed incidents: %w", err)
	}

	if len(incidents) == 0 {
		fmt.Println("No unprocessed incidents found.")
		return nil
	}

	councillors, err := db.GetCouncillorInfo()
	if err != nil {
		return fmt.Errorf("get councillor info failure: %w", err)
	}

	fmt.Printf("Processing %d unprocessed incident(s)...\n\n", len(incidents))

	for _, incident := range incidents {
		// Parse the start_dt to get the year
		id := incident["id"].(string)
		startDt := incident["start_dt"].(string) // UTC, no offset suffix; time.Parse defaults to UTC.
		incidentInfo := incident["incident_info"].(string)
		communityName := incident["community_name"].(string)
		wardNum := incident["ward_num"].(string)
		councillorName := incident["councillor_name"].(string)
		rowNumber := incident["row_number"].(string)

		// Parse the date
		incidentDate, err := time.Parse("2006-01-02T15:04:05.000", startDt)
		if err != nil {
			log.Printf("Warning: Failed to parse date for incident %s: %v", id, err)
		}
		year := incidentDate.In(mountain).Year()

		// String to int
		row, err := strconv.Atoi(rowNumber)

		// Find which number this was for the year and ward
		wardCount, err := db.CountPedestrianIncidentsByWardAndYear(id, wardNum, year)
		if err != nil {
			return fmt.Errorf("count incidents for ward %s: %w", wardNum, err)
		}

		// Format the message
		message := formatIncidentMessage(row, incidentDate.In(mountain), incidentInfo, communityName,
			councillorName, wardCount, wardNum)

		// Print the formatted message
		fmt.Println(message)
		fmt.Println(strings.Repeat("-", 80))
		fmt.Println()

		// Post to Social Media
		enableBluesky := os.Getenv("ENABLE_BLUESKY")
		enableMastodon := os.Getenv("ENABLE_MASTODON")
		enableThreads := os.Getenv("ENABLE_THREADS")
		enableX := os.Getenv("ENABLE_X")

		// Concurrency Management
		g, ctx := errgroup.WithContext(context.Background())

		if enableBluesky != "" && enableBluesky == "TRUE" {
			g.Go(func() error {
				// Tweak message for Bluesky
				blueSkyMessage := formatMessageForBluesky(message, councillors, wardNum)

				if err := postToBluesky(ctx, blueSkyMessage); err != nil {
					return err
				}
				return nil
			})
		}

		if enableMastodon != "" && enableMastodon == "TRUE" {
			g.Go(func() error {
				// Tweak message for Mastodon
				mastodonMessage := formatMessageForMastodon(message, councillors, wardNum)

				if err := postToMastodon(ctx, mastodonMessage); err != nil {
					return err
				}
				return nil
			})
		}

		if enableThreads != "" && enableThreads == "TRUE" {
			g.Go(func() error {
				// Tweak message for Threads
				threadsMessage := formatMessageForThreads(message, councillors, wardNum)

				if err := postToThreads(ctx, threadsMessage); err != nil {
					return err
				}
				return nil
			})
		}

		if enableX != "" && enableX == "TRUE" {
			g.Go(func() error {
				// X sucks and 403s with mentions now.
				xMessage := message
				// Tweak message for X
				//xMessage := formatMessageForX(message, councillors, wardNum)

				if err := postToX(ctx, xMessage); err != nil {
					return err
				}
				return nil
			})
		}

		// Mark as processed
		if err := db.MarkAsProcessed(id); err != nil {
			return fmt.Errorf("mark incident as processed: %w", err)
		}

		if err := g.Wait(); err != nil {
			fmt.Printf("Error: %s\n", err)
		}
	}

	fmt.Printf("Successfully processed %d incident(s).\n", len(incidents))
	return nil
}

// formatIncidentMessage formats an incident according to the template
func formatIncidentMessage(rowNumber int, incidentDate time.Time, incidentInfo, communityName,
	councillorName string, wardCount int, wardNum string) string {

	var sb strings.Builder

	// First line: "This was the Xth pedestrian incident in YYYY."
	sb.WriteString(fmt.Sprintf("The %s pedestrian incident in %d happened on %s at %s.\n",
		ordinalSuffix(rowNumber), incidentDate.Year(), incidentDate.Format("Jan 02"), incidentDate.Format("15:04")))

	// Second line: Location information - trim double spaces and end text
	caser := cases.Title(language.English)
	sb.WriteString(fmt.Sprintf("It took place at %s in %s.\n",
		strings.TrimSpace(strings.ReplaceAll(incidentInfo, "  ", " ")), caser.String(communityName)))

	// Third line: Ward details
	if wardCount > 0 {
		sb.WriteString(fmt.Sprintf("\nThis was the %s incident in ward %s this year.\n",
			ordinalSuffix(wardCount), wardNum))
	}

	// Hashtags
	sb.WriteString("\n#yyccc #yycwalk")

	return sb.String()
}

// formatMessageForBluesky replaces councillorName with Bluesky handle if it exists
func formatMessageForBluesky(message string, councillors map[string]*Councillor, wardNum string) string {

	councillor, exists := councillors[wardNum]
	if !exists || councillor.BlueskyHandle == "" {
		return message
	}

	return message + fmt.Sprintf("\n@%s", councillor.BlueskyHandle)
}

// formatMessageForMastodon replaces councillorName with Mastodon handle if it exists
func formatMessageForMastodon(message string, councillors map[string]*Councillor, wardNum string) string {
	councillor, exists := councillors[wardNum]
	if !exists || councillor.MastodonHandle == "" {
		return message
	}

	return message + fmt.Sprintf("\n@%s", councillor.MastodonHandle)
}

// formatMessageForThreads replaces councillorName with Threads handle if it exists
func formatMessageForThreads(message string, councillors map[string]*Councillor, wardNum string) string {
	councillor, exists := councillors[wardNum]
	if !exists || councillor.ThreadsHandle == "" {
		return message
	}

	return message + fmt.Sprintf("\n@%s", councillor.ThreadsHandle)
}

// formatMessageForX replaces councillorName with X handle if it exists
func formatMessageForX(message string, councillors map[string]*Councillor, wardNum string) string {
	councillor, exists := councillors[wardNum]

	if !exists || councillor.XHandle == "" {
		return message
	}

	return message + fmt.Sprintf("\n@%s", councillor.XHandle)
}

// ordinalSuffix returns a number with its ordinal suffix (1st, 2nd, 3rd, 4th, etc.)
func ordinalSuffix(n int) string {
	// Handle special cases for 11th, 12th, 13th
	if n%100 >= 11 && n%100 <= 13 {
		return fmt.Sprintf("%dth", n)
	}

	// Handle other cases
	switch n % 10 {
	case 1:
		return fmt.Sprintf("%dst", n)
	case 2:
		return fmt.Sprintf("%dnd", n)
	case 3:
		return fmt.Sprintf("%drd", n)
	default:
		return fmt.Sprintf("%dth", n)
	}
}
