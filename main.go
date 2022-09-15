package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
)

type Place struct {
	Number      int
	BikeNumbers []string `json:"bike_numbers"`
	Longitude   float64  `json:"lng"`
	Latitude    float64  `json:"lat"`
}

type City struct {
	Name   string  `json:"name"`
	Places []Place `json:"places"`
}

type Country struct {
	Name   string `json:"country_name"`
	Cities []City `json:"cities"`
}

type Data struct {
	Countries []Country `json:"countries"`
}

func main() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	db, err := sql.Open("sqlite3", "./nextbike.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	sqlStmt := `
		CREATE TABLE IF NOT EXISTS nextbike (
			id integer not null primary key autoincrement,
			bike_id integer not null,
			latitude real,
			longitude real,
			seen_at date default (datetime('now','localtime')) not null
		);
		CREATE INDEX IF NOT EXISTS idx_bike_id_seen_at
		ON nextbike (bike_id, seen_at);
	`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}
	NextbikeTracker(db)
	for range time.Tick(5 * time.Minute) {
		NextbikeTracker(db)
	}
}

func NextbikeTracker(db *sql.DB) {
	log.Info("start")
	ctx := context.Background()

	eCargoBikes := []string{
		"20091",
		"20095", "20096", "20111",
		"20118", "20119",
	}

	client := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   time.Second * 5,
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", "https://maps.nextbike.net/maps/nextbike-live.json?city=1&domains=le&list_cities=0&bikes=0", nil)

	res, err := client.Do(req)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("error fetching JSON")
		return
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("error reading body")
		return
	}

	err = res.Body.Close()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("error closing body")
		return
	}

	var nbd Data

	err = json.Unmarshal(body, &nbd)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("error unmarshalling JSON")
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare("INSERT INTO nextbike(bike_id, latitude, longitude) values(?, ?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	for _, c := range nbd.Countries {
		if c.Name == "Germany" {
			for _, city := range c.Cities {
				if city.Name == "Leipzig" {
					for _, place := range city.Places {
						for _, bn := range place.BikeNumbers {
							for _, e := range eCargoBikes {
								if e == bn {
									_, err = stmt.Exec(bn, place.Latitude, place.Longitude)
								}
							}
						}
					}
				}
			}
		}
	}

	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}

	log.Info("finished")
}
