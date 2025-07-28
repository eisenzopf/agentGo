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
	file, err := os.Open("mouse_movements.csv")
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

	// Get logical screen dimensions for playback
	logicalWidth, logicalHeight := robotgo.GetScreenSize()
	log.Printf("Playing back on logical screen size: %d x %d", logicalWidth, logicalHeight)

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
		normX, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			log.Printf("failed to parse normalized x coordinate: %v", err)
			continue
		}
		normY, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			log.Printf("failed to parse normalized y coordinate: %v", err)
			continue
		}

		// Wait for the correct amount of time
		if i > 0 {
			delay := time.Duration(timestamp-lastTimestamp) * time.Millisecond
			time.Sleep(delay)
		}
		lastTimestamp = timestamp

		// De-normalize the coordinates for the current screen and move the mouse
		finalX := int(normX * float64(logicalWidth))
		finalY := int(normY * float64(logicalHeight))
		
		fmt.Printf("Moving mouse to (%d, %d) (Normalized: %.4f, %.4f)\n", finalX, finalY, normX, normY)
		robotgo.Move(finalX, finalY)
	}

	log.Println("Playback finished.")
}