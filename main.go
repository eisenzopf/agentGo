package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"image/png"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/google/generative-ai-go/genai"
	"github.com/kbinani/screenshot"
	"google.golang.org/api/option"
)

const recordingTime = 30 * time.Second

func main() {
	// Get API key from environment variable
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable not set")
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
	log.Printf("Scaling factors: x=%.2f, y=%.2f", xScale, yScale)


	// Create a new Gemini client
	ctx, cancel := context.WithTimeout(context.Background(), recordingTime)
	defer cancel()

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Initialize the generative model
	model := client.GenerativeModel("gemini-1.5-flash")

	// Create and open the CSV file
	file, err := os.Create("mouse_movements.csv")
	if err != nil {
		log.Fatalf("failed to create csv file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header
	if err := writer.Write([]string{"timestamp", "x", "y"}); err != nil {
		log.Fatalf("failed to write header to csv: %v", err)
	}

	log.Println("Starting to record mouse movements for 30 seconds...")

	// Main loop to capture screen and get cursor position
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			log.Println("Recording finished.")
			return
		case t := <-ticker.C:
			// Capture the screen
			img, err := screenshot.CaptureRect(bounds)
			if err != nil {
				log.Printf("failed to capture screen: %v", err)
				continue
			}

			// Encode the image to PNG
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				log.Printf("failed to encode image: %v", err)
				continue
			}

			// Send the image to Gemini
			prompt := "Find the mouse cursor in this image and return its x,y coordinates. For example: 123,456"
			res, err := model.GenerateContent(ctx, genai.Text(prompt), genai.ImageData("png", buf.Bytes()))
			if err != nil {
				log.Printf("failed to generate content: %v", err)
				continue
			}

			// Extract and print the coordinates from the response
			if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
				if coordsText, ok := res.Candidates[0].Content.Parts[0].(genai.Text); ok {
					coords := strings.Split(strings.TrimSpace(string(coordsText)), ",")
					if len(coords) == 2 {
						geminiX, errX := strconv.Atoi(coords[0])
						geminiY, errY := strconv.Atoi(coords[1])

						if errX == nil && errY == nil {
							// Scale the coordinates
							finalX := int(float64(geminiX) * xScale)
							finalY := int(float64(geminiY) * yScale)

							timestamp := t.Sub(startTime).Milliseconds()
							record := []string{fmt.Sprintf("%d", timestamp), fmt.Sprintf("%d", finalX), fmt.Sprintf("%d", finalY)}
							if err := writer.Write(record); err != nil {
								log.Printf("failed to write record to csv: %v", err)
							}
							fmt.Printf("Recorded Scaled Coords: %v (Original: %s,%s)\n", record, coords[0], coords[1])
						}
					}
				}
			}
		}
	}
}
