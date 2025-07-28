package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/kbinani/screenshot"
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

	// Get screen dimensions for scaling
	logicalWidth, logicalHeight := robotgo.GetScreenSize()
	bounds := screenshot.GetDisplayBounds(0)
	physicalWidth := bounds.Dx()
	physicalHeight := bounds.Dy()

	if physicalWidth == 0 || physicalHeight == 0 {
		log.Fatal("Could not get physical screen dimensions.")
	}

	xScale := float64(logicalWidth) / float64(physicalWidth)
	yScale := float64(logicalHeight) / float64(physicalHeight)

	log.Printf("Physical (Screenshot) Dimensions: %d x %d", physicalWidth, physicalHeight)
	log.Printf("Logical (Mouse) Dimensions: %d x %d", logicalWidth, logicalHeight)
	log.Printf("Applying scaling factors: x=%.2f, y=%.2f", xScale, yScale)

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
		rawX, err := strconv.Atoi(record[1])
		if err != nil {
			log.Printf("failed to parse x coordinate: %v", err)
			continue
		}
		rawY, err := strconv.Atoi(record[2])
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

		// Scale the coordinates and move the mouse
		finalX := int(float64(rawX) * xScale)
		finalY := int(float64(rawY) * yScale)
		fmt.Printf("Moving mouse to (%d, %d) (Raw: %d,%d)\n", finalX, finalY, rawX, rawY)
		robotgo.Move(finalX, finalY)
	}

	log.Println("Playback finished.")
}
