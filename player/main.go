package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-vgo/robotgo"
)

func main() {
	// Open the CSV file
	file, err := os.Open("../mouse_movements.csv")
	if err != nil {
		log.Fatalf("failed to open csv file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("failed to read csv records: %v", err)
	}

	// Remove header row
	if len(records) > 0 {
		records = records[1:]
	}

	log.Println("Starting mouse playback...")

	var lastTimestamp int64

	for i, record := range records {
		if len(record) != 3 {
			log.Printf("skipping malformed record: %v", record)
			continue
		}

		// Parse the record
		timestamp, err := strconv.ParseInt(record[0], 10, 64)
		if err != nil {
			log.Printf("failed to parse timestamp: %v", err)
			continue
		}
		x, err := strconv.Atoi(record[1])
		if err != nil {
			log.Printf("failed to parse x coordinate: %v", err)
			continue
		}
		y, err := strconv.Atoi(record[2])
		if err != nil {
			log.Printf("failed to parse y coordinate: %v", err)
			continue
		}

		// Wait for the correct amount of time
		if i > 0 {
			delay := time.Duration(timestamp-lastTimestamp) * time.Millisecond
			time.Sleep(delay)
		}
		lastTimestamp = timestamp

		// Move the mouse
		fmt.Printf("Moving mouse to (%d, %d)\n", x, y)
		robotgo.Move(x, y)
	}

	log.Println("Playback finished.")
}