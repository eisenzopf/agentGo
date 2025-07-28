package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
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
	bounds := screenshot.GetDisplayBounds(0)

	// Get scaling factor for drawing mouse on screenshot
	logicalWidth, logicalHeight := robotgo.GetScreenSize()
	physicalWidth := bounds.Dx()
	physicalHeight := bounds.Dy()
	xScale := float64(physicalWidth) / float64(logicalWidth)
	yScale := float64(physicalHeight) / float64(logicalHeight)

	for {
		select {
		case <-ctx.Done():
			log.Println("Recording finished.")
			return
		case t := <-ticker.C:
			// Get mouse position
			mouseX, mouseY := robotgo.GetMousePos()

			// Capture the screen
			img, err := screenshot.CaptureRect(bounds)
			if err != nil {
				log.Printf("failed to capture screen: %v", err)
				continue
			}

			// Create a new RGBA image to draw on
			drawableImg := image.NewRGBA(img.Bounds())
			draw.Draw(drawableImg, drawableImg.Bounds(), img, image.Point{}, draw.Src)

			// Calculate the position to draw the cursor on the physical screenshot
			drawX := int(float64(mouseX) * xScale)
			drawY := int(float64(mouseY) * yScale)

			// Draw a red square to represent the cursor
			cursorColor := color.RGBA{R: 255, G: 0, B: 0, A: 255}
			cursorSize := 15
			draw.Draw(drawableImg, image.Rect(drawX, drawY, drawX+cursorSize, drawY+cursorSize), &image.Uniform{C: cursorColor}, image.Point{}, draw.Src)

			// Encode the modified image to PNG
			var buf bytes.Buffer
			if err := png.Encode(&buf, drawableImg); err != nil {
				log.Printf("failed to encode image: %v", err)
				continue
			}

			// --- DEBUG: Save the screenshot to a file with coordinates ---
			debugFilename := fmt.Sprintf("debug_x%d_y%d_t%d.png", drawX, drawY, t.Unix())
			if err := os.WriteFile(debugFilename, buf.Bytes(), 0644); err != nil {
				log.Printf("failed to create debug file: %v", err)
			}
			// --- END DEBUG ---
			
			// Send the image to Gemini
			prompt := "Find the red square in this image and return its top-left x,y coordinates. For example: 123,456"
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
						timestamp := t.Sub(startTime).Milliseconds()
						record := []string{fmt.Sprintf("%d", timestamp), coords[0], coords[1]}
						if err := writer.Write(record); err != nil {
							log.Printf("failed to write record to csv: %v", err)
						}
						fmt.Printf("Recorded Raw Coords: %v\n", record)
					}
				}
			}
		}
	}
}