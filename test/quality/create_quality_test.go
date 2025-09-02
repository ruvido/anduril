package main

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
)

func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Create a detailed pattern to better show compression differences
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create a complex pattern with gradients and details
			r := uint8((x * 255) / width)
			g := uint8((y * 255) / height)
			b := uint8((x + y*2) % 255)
			
			// Add some noise/detail for compression testing
			if (x+y)%3 == 0 {
				r = 255 - r
			}
			if (x*y)%7 == 0 {
				g = 255 - g
			}
			
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	
	return img
}

func saveImageWithQuality(img image.Image, path string, quality int) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	options := &jpeg.Options{Quality: quality}
	return jpeg.Encode(file, img, options)
}

func main() {
	// Create same image with different qualities
	img := createTestImage(800, 600)
	
	// Save with different qualities
	qualities := []struct {
		filename string
		quality  int
	}{
		{"test_photo_high.jpg", 95},    // Highest quality
		{"test_photo_medium.jpg", 80},  // Medium quality  
		{"test_photo_low.jpg", 50},     // Low quality
		{"test_photo_worst.jpg", 20},   // Very low quality
	}
	
	for _, q := range qualities {
		if err := saveImageWithQuality(img, q.filename, q.quality); err != nil {
			fmt.Printf("Error saving %s: %v\n", q.filename, err)
		} else {
			fmt.Printf("Created %s with quality %d\n", q.filename, q.quality)
		}
	}
	
	// Create different resolution versions
	resolutions := []struct {
		filename string
		width, height int
		quality int
	}{
		{"test_photo_1920x1080.jpg", 1920, 1080, 85},
		{"test_photo_1280x720.jpg", 1280, 720, 85},
		{"test_photo_640x480.jpg", 640, 480, 85},
		{"test_photo_320x240.jpg", 320, 240, 85},
	}
	
	for _, r := range resolutions {
		resImg := createTestImage(r.width, r.height)
		if err := saveImageWithQuality(resImg, r.filename, r.quality); err != nil {
			fmt.Printf("Error saving %s: %v\n", r.filename, err)
		} else {
			fmt.Printf("Created %s with resolution %dx%d\n", r.filename, r.width, r.height)
		}
	}
	
	fmt.Println("\nQuality test files created!")
	fmt.Println("Use: anduril import test/quality --dry-run to test quality comparison")
}