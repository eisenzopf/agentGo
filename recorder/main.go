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
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/google/generative-ai-go/genai"
	"github.com/kbinani/screenshot"
	"google.golang.org/api/option"
)

const recordingTime = 10 * time.Second

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
	if err := writer.Write([]string{"timestamp", "norm_x", "norm_y"}); err != nil {
		log.Fatalf("failed to write header to csv: %v", err)
	}

	log.Println("Starting to record mouse movements for 30 seconds...")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	bounds := screenshot.GetDisplayBounds(0)
	
	// Get screen dimensions
	logicalWidth, logicalHeight := robotgo.GetScreenSize()
	physicalWidth := bounds.Dx()
	physicalHeight := bounds.Dy()

	// Get scaling factor for drawing mouse on screenshot
	xScale := float64(physicalWidth) / float64(logicalWidth)
	yScale := float64(physicalHeight) / float64(logicalHeight)

	for {
		select {
		case <-ctx.Done():
			log.Println("Recording finished.")
			return
		case t := <-ticker.C:
			// --- Step 1: Get mouse position directly and normalize it ---
			mouseX, mouseY := robotgo.GetMousePos()
			normX := float64(mouseX) / float64(logicalWidth)
			normY := float64(mouseY) / float64(logicalHeight)

			// --- Step 2: Write the directly-sourced coordinates to the CSV ---
			timestamp := t.Sub(startTime).Milliseconds()
			record := []string{
				fmt.Sprintf("%d", timestamp),
				fmt.Sprintf("%.8f", normX),
				fmt.Sprintf("%.8f", normY),
			}
			if err := writer.Write(record); err != nil {
				log.Printf("failed to write record to csv: %v", err)
			}
			fmt.Printf("Recorded Normalized Coords: %v\n", record)

			// --- Step 3: Continue with visual analysis for debugging and future use ---
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
			
			// Save a debug screenshot
			debugFilename := fmt.Sprintf("debug_x%d_y%d_t%d.png", drawX, drawY, t.Unix())
			if err := os.WriteFile(debugFilename, buf.Bytes(), 0644); err != nil {
				log.Printf("failed to create debug file: %v", err)
			}

			// Send the image to Gemini (its response is not used for recording anymore, but is useful for future steps)
			prompt := "Find the red square in this image and return its top-left x,y coordinates. For example: 123,456"
			go func() {
				_, err := model.GenerateContent(ctx, genai.Text(prompt), genai.ImageData("png", buf.Bytes()))
				if err != nil {
					log.Printf("Gemini call failed (this is for info only): %v", err)
				}
			}()
		}
	}
}