package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Traffic Incidents - https://data.calgary.ca/Transportation-Transit/Traffic-Incidents/35ra-9556/
// *Start* Date and times are stored in UTC.
// *Modified* date and times are MT.
type Incident struct {
	ID           string  `json:"id"`
	IncidentInfo string  `json:"incident_info"`
	Description  string  `json:"description"`
	StartDt      string  `json:"start_dt"`
	ModifiedDt   string  `json:"modified_dt"`
	Quadrant     string  `json:"quadrant"`
	Longitude    float64 `json:"longitude,string"`
	Latitude     float64 `json:"latitude,string"`
	CommunityID  string  // Community code from geo calculation (not from API)
	WardID       string  // Ward number from geo calculation (not from API)
	Processed    bool    `json:"-"`
}

// Community - https://data.calgary.ca/Government/Community-District-Boundaries/ab7m-fwn6/
type Community struct {
	CommCode     string          `json:"comm_code"`
	Name         string          `json:"name"`
	Multipolygon json.RawMessage `json:"multipolygon"`
}

// Ward - https://data.calgary.ca/Government/Ward-Boundaries-2023-/tz8z-hyaz/
type Ward struct {
	WardNum      string          `json:"ward_num"`
	Multipolygon json.RawMessage `json:"multipolygon"`
}

// Councillor
type Councillor struct {
	WardNum        string `json:"ward"`
	Name           string `json:"councillor"`
	BlueSkyHandle  string `json:"bluesky_handle"`
	MastodonHandle string `json:"mastodon_handle"`
	ThreadsHandle  string `json:"threads_handle"`
	XHandle        string `json:"x_handle"`
}

// IncidentDB manages the SQLite database for traffic incidents
type IncidentDB struct {
	db *sql.DB
}

// NewIncidentDB creates a new IncidentDB instance and opens a connection to the database
func NewIncidentDB(dbPath string) (*IncidentDB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", dbPath, err)
	}

	db.SetConnMaxLifetime(time.Hour)

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &IncidentDB{db: db}, nil
}

func (tdb *IncidentDB) CreateSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS traffic_incidents (
			id TEXT PRIMARY KEY,
			incident_info TEXT NOT NULL,
			description TEXT NOT NULL,
			start_dt TEXT NOT NULL,
			modified_dt TEXT NOT NULL,
			quadrant TEXT,
			longitude REAL,
			latitude REAL,
			community_id TEXT NOT NULL DEFAULT '',
			ward_id TEXT NOT NULL DEFAULT '',
			processed INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);

		CREATE INDEX IF NOT EXISTS idx_start_dt ON traffic_incidents(start_dt);
		CREATE INDEX IF NOT EXISTS idx_modified_dt ON traffic_incidents(modified_dt);

		CREATE TABLE IF NOT EXISTS communities (
			comm_code TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			multipolygon TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS councillors (
			ward_num TEXT PRIMARY KEY,
			councillor_name TEXT NOT NULL,
			councillor_bluesky TEXT,
			councillor_mastodon TEXT,
			councillor_threads TEXT,
			councillor_x TEXT
		);

		CREATE TABLE IF NOT EXISTS wards (
			ward_num TEXT PRIMARY KEY,
			multipolygon TEXT NOT NULL
		);
	`

	if _, err := tdb.db.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	return nil
}

// UpsertIncident inserts a new incident or updates an existing one
func (tdb *IncidentDB) UpsertIncident(incident *Incident) error {
	if incident.ID == "" {
		return fmt.Errorf("incident ID cannot be empty")
	}

	query := `
		INSERT INTO traffic_incidents (
			id, incident_info, description, start_dt, modified_dt,
			quadrant, longitude, latitude,
			community_id, ward_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			incident_info = excluded.incident_info,
			description = excluded.description,
			modified_dt = excluded.modified_dt,
			quadrant = excluded.quadrant,
			longitude = excluded.longitude,
			latitude = excluded.latitude,
			community_id = excluded.community_id,
			ward_id = excluded.ward_id,
			updated_at = datetime('now')
	`

	_, err := tdb.db.Exec(query,
		incident.ID,
		incident.IncidentInfo,
		incident.Description,
		incident.StartDt,
		incident.ModifiedDt,
		incident.Quadrant,
		incident.Longitude,
		incident.Latitude,
		incident.CommunityID,
		incident.WardID,
	)
	if err != nil {
		return fmt.Errorf("upsert incident %s: %w", incident.ID, err)
	}

	return nil
}

func (tdb *IncidentDB) GetUnprocessedIncidentCount() (int, error) {
	query := `
		SELECT count(*)
		FROM traffic_incidents
		WHERE processed=0
		`

	var count int
	err := tdb.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count pedestrian incidents by ward: %w", err)
	}

	return count, nil
}

func (tdb *IncidentDB) GetUnprocessedIncidents() ([]map[string]interface{}, error) {
	// Double query in order to set a row number and avoid a 6 hour pain point around New Year's...
	query := `
		SELECT
			id,
			start_dt,
			incident_info,
			description,
			quadrant,
			row_number,
			community_name,
			ward_num,
			councillor_name
		FROM
		  (
				SELECT
					traffic_incidents.id,
					traffic_incidents.start_dt,
					traffic_incidents.incident_info,
					traffic_incidents.description,
					traffic_incidents.quadrant,
					traffic_incidents.processed,
					-- Manually set time for -6 hours which is Alberta time
					row_number () OVER ( PARTITION BY strftime('%Y', start_dt, '-6 hours') ORDER BY start_dt ) as row_number,
					COALESCE(communities.name, 'Unknown Community') as community_name,
					COALESCE(traffic_incidents.ward_id, '') as ward_num,
					COALESCE(councillors.councillor_name, '') as councillor_name
				FROM traffic_incidents
				LEFT JOIN communities ON traffic_incidents.community_id = communities.comm_code
				LEFT JOIN councillors ON traffic_incidents.ward_id = councillors.ward_num
			)
		WHERE processed = 0
	`

	rows, err := tdb.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query unprocessed enriched: %w", err)
	}
	defer rows.Close()

	var incidents []map[string]interface{}
	for rows.Next() {
		var id, startDt, incidentInfo, description, quadrant string
		var rowNumber, communityName, wardNum, councillorName string

		err := rows.Scan(&id, &startDt, &incidentInfo, &description, &quadrant,
			&rowNumber, &communityName, &wardNum, &councillorName)
		if err != nil {
			return nil, fmt.Errorf("scan enriched incident row: %w", err)
		}

		incident := map[string]interface{}{
			"id":              id,
			"start_dt":        startDt,
			"incident_info":   incidentInfo,
			"description":     description,
			"quadrant":        quadrant,
			"community_name":  communityName,
			"ward_num":        wardNum,
			"councillor_name": councillorName,
			"row_number":      rowNumber,
		}

		incidents = append(incidents, incident)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return incidents, nil
}

// CountPedestrianIncidentsByWardAndYear counts pedestrian incidents in a ward for a given year
// We can use ID to filter since we have it queried before
func (tdb *IncidentDB) CountPedestrianIncidentsByWardAndYear(id string, wardID string, year int) (int, error) {
	if wardID == "" {
		return 0, nil
	}

	query := `
		SELECT COUNT(*)+1
		FROM traffic_incidents
		WHERE ward_id = ?
		AND strftime('%Y', start_dt, '-6 hours') = ?
		AND processed = 1
	`

	var count int
	err := tdb.db.QueryRow(query, wardID, fmt.Sprintf("%d", year), id).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count pedestrian incidents by ward: %w", err)
	}

	return count, nil
}

// MarkAsProcessed marks an incident as processed
func (tdb *IncidentDB) MarkAsProcessed(incidentID string) error {
	if incidentID == "" {
		return fmt.Errorf("incident ID cannot be empty")
	}

	query := `
		UPDATE traffic_incidents
		SET processed = 1, updated_at = datetime('now')
		WHERE id = ?
	`

	result, err := tdb.db.Exec(query, incidentID)
	if err != nil {
		return fmt.Errorf("mark incident %s as processed: %w", incidentID, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("incident %s not found", incidentID)
	}

	return nil
}

// UpsertCommunity inserts a new community or updates an existing one
func (tdb *IncidentDB) UpsertCommunity(community *Community) error {
	if community.CommCode == "" {
		return fmt.Errorf("community comm_code cannot be empty")
	}

	query := `
		INSERT INTO communities (comm_code, name, multipolygon)
		VALUES (?, ?, ?)
		ON CONFLICT(comm_code) DO UPDATE SET
			name = excluded.name,
			multipolygon = excluded.multipolygon
	`

	_, err := tdb.db.Exec(query, community.CommCode, community.Name, string(community.Multipolygon))
	if err != nil {
		return fmt.Errorf("upsert community %s: %w", community.CommCode, err)
	}

	return nil
}

// UpsertCouncillor inserts a new ward or updates an existing one
func (tdb *IncidentDB) UpsertCouncillor(councillor *Councillor) error {
	if councillor.WardNum == "" {
		return fmt.Errorf("councillor's ward id cannot be empty")
	}

	query := `
		INSERT INTO councillors (ward_num, councillor_name, councillor_bluesky, councillor_mastodon, councillor_threads, councillor_x)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(ward_num) DO UPDATE SET
		  councillor_name = excluded.councillor_name,
			councillor_bluesky = excluded.councillor_bluesky,
		  councillor_mastodon = excluded.councillor_mastodon,
			councillor_threads = excluded.councillor_threads,
			councillor_x = excluded.councillor_x
	`

	_, err := tdb.db.Exec(query, councillor.WardNum, councillor.Name, councillor.BlueSkyHandle, councillor.MastodonHandle, councillor.ThreadsHandle, councillor.XHandle)
	if err != nil {
		return fmt.Errorf("upsert councillor %s: %w", councillor.WardNum, err)
	}

	return nil
}

// UpsertWard inserts a new ward or updates an existing one
func (tdb *IncidentDB) UpsertWard(ward *Ward) error {
	if ward.WardNum == "" {
		return fmt.Errorf("ward number cannot be empty")
	}

	query := `
		INSERT INTO wards (ward_num, multipolygon)
		VALUES (?, ?)
		ON CONFLICT(ward_num) DO UPDATE SET
			multipolygon = excluded.multipolygon
	`

	_, err := tdb.db.Exec(query, ward.WardNum, string(ward.Multipolygon))
	if err != nil {
		return fmt.Errorf("upsert ward %s: %w", ward.WardNum, err)
	}

	return nil
}

// GetWards retrieves all wards from the database
func (tdb *IncidentDB) GetWards() ([]Ward, error) {
	query := `SELECT ward_num, multipolygon FROM wards`

	rows, err := tdb.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("get wards: %w", err)
	}
	defer rows.Close()

	var wards []Ward
	for rows.Next() {
		var ward Ward
		var multipolygonStr string
		if err := rows.Scan(&ward.WardNum, &multipolygonStr); err != nil {
			return nil, fmt.Errorf("scan ward row: %w", err)
		}
		ward.Multipolygon = json.RawMessage(multipolygonStr)
		wards = append(wards, ward)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ward rows: %w", err)
	}

	return wards, nil
}

// GetCommunities retrieves all communities from the database
func (tdb *IncidentDB) GetCommunities() ([]Community, error) {
	query := `SELECT comm_code, name, multipolygon FROM communities`

	rows, err := tdb.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("get communities: %w", err)
	}
	defer rows.Close()

	var communities []Community
	for rows.Next() {
		var community Community
		var multipolygonStr string
		if err := rows.Scan(&community.CommCode, &community.Name, &multipolygonStr); err != nil {
			return nil, fmt.Errorf("scan community row: %w", err)
		}
		community.Multipolygon = json.RawMessage(multipolygonStr)
		communities = append(communities, community)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate community rows: %w", err)
	}

	return communities, nil
}

// GetCouncillorInfo retrieves all councillors and returns them as a map keyed by ward number
func (tdb *IncidentDB) GetCouncillorInfo() (map[string]*Councillor, error) {
	query := `
		SELECT ward_num, councillor_name, councillor_bluesky, councillor_mastodon, councillor_threads, councillor_x
		FROM councillors
	`

	rows, err := tdb.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("get councillor info: %w", err)
	}
	defer rows.Close()

	councillors := make(map[string]*Councillor)
	for rows.Next() {
		var councillor Councillor
		err := rows.Scan(
			&councillor.WardNum,
			&councillor.Name,
			&councillor.BlueSkyHandle,
			&councillor.MastodonHandle,
			&councillor.ThreadsHandle,
			&councillor.XHandle,
		)
		if err != nil {
			return nil, fmt.Errorf("scan councillor row: %w", err)
		}

		councillors[councillor.WardNum] = &councillor
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate councillor rows: %w", err)
	}

	return councillors, nil
}

// Close closes the database connection
func (tdb *IncidentDB) Close() error {
	if err := tdb.db.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	return nil
}
