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
	"strconv"
	"strings"
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

	// Create and open the CSV file for the player
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

	log.Println("Starting to record mouse movements for 10 seconds...")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	bounds := screenshot.GetDisplayBounds(0)
	
	// Get screen dimensions
	logicalWidth, logicalHeight := robotgo.GetScreenSize()
	physicalWidth := float64(bounds.Dx())
	physicalHeight := float64(bounds.Dy())

	// Get scaling factor for drawing mouse on screenshot
	xScale := float64(bounds.Dx()) / float64(logicalWidth)
	yScale := float64(bounds.Dy()) / float64(logicalHeight)

	for {
		select {
		case <-ctx.Done():
			log.Println("Recording finished.")
			return
		case t := <-ticker.C:
			// --- Step 1: Get GROUND TRUTH mouse position and normalize it ---
			mouseX, mouseY := robotgo.GetMousePos()
			groundTruthNormX := float64(mouseX) / float64(logicalWidth)
			groundTruthNormY := float64(mouseY) / float64(logicalHeight)

			// --- Step 2: Write the ground truth coordinates to the CSV for the player ---
			timestamp := t.Sub(startTime).Milliseconds()
			record := []string{
				fmt.Sprintf("%d", timestamp),
				fmt.Sprintf("%.8f", groundTruthNormX),
				fmt.Sprintf("%.8f", groundTruthNormY),
			}
			if err := writer.Write(record); err != nil {
				log.Printf("failed to write record to csv: %v", err)
			}

			// --- Step 3: Perform visual analysis to get Gemini's coordinates ---
			img, err := screenshot.CaptureRect(bounds)
			if err != nil {
				log.Printf("failed to capture screen: %v", err)
				continue
			}

			// The image from screenshot is already an *image.RGBA, so we can draw on it directly.
			drawX := int(float64(mouseX) * xScale)
			drawY := int(float64(mouseY) * yScale)

			// Draw a red crosshair to represent the cursor
			cursorColor := color.RGBA{R: 255, G: 0, B: 0, A: 255}
			armLength := 15 // 15px out from the center
			thickness := 3  // 3px thick lines
			// Horizontal line
			draw.Draw(img, image.Rect(drawX-armLength, drawY-thickness/2, drawX+armLength, drawY+thickness/2), &image.Uniform{C: cursorColor}, image.Point{}, draw.Src)
			// Vertical line
			draw.Draw(img, image.Rect(drawX-thickness/2, drawY-armLength, drawX+thickness/2, drawY+armLength), &image.Uniform{C: cursorColor}, image.Point{}, draw.Src)

			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				log.Printf("failed to encode image: %v", err)
				continue
			}

			// Save a debug screenshot
			debugFilename := fmt.Sprintf("debug_x%d_y%d_t%d.png", drawX, drawY, t.Unix())
			if err := os.WriteFile(debugFilename, buf.Bytes(), 0644); err != nil {
				log.Printf("failed to create debug file: %v", err)
			}
			
			// Send the image to Gemini with the improved prompt
			prompt := "This screenshot has an artificial red crosshair marker drawn on it. Your task is to ignore all other UI elements and find this red crosshair. Return only the center x,y coordinates of the crosshair in the format x,y."
			res, err := model.GenerateContent(ctx, genai.Text(prompt), genai.ImageData("png", buf.Bytes()))
			if err != nil {
				log.Printf("Gemini call failed: %v", err)
				continue
			}

			// --- Step 4: Compare Gemini's response to the ground truth ---
			var geminiNormX, geminiNormY float64
			geminiCoordsStr := "N/A"
			if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
				if coordsText, ok := res.Candidates[0].Content.Parts[0].(genai.Text); ok {
					coords := strings.Split(strings.TrimSpace(string(coordsText)), ",")
					geminiCoordsStr = string(coordsText)
					if len(coords) == 2 {
						geminiX, errX := strconv.ParseFloat(coords[0], 64)
						geminiY, errY := strconv.ParseFloat(coords[1], 64)

						if errX == nil && errY == nil {
							geminiNormX = geminiX / physicalWidth
							geminiNormY = geminiY / physicalHeight
						}
					}
				}
			}
			
			log.Printf(
				"Ground Truth: (%.4f, %.4f) vs Gemini: (%.4f, %.4f) [Raw Gemini: %s]",
				groundTruthNormX, groundTruthNormY,
				geminiNormX, geminiNormY,
				geminiCoordsStr,
			)
		}
	}
}
