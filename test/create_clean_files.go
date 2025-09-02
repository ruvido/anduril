package main

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
)

func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x * 255) / width)
			g := uint8((y * 255) / height)
			b := uint8((x + y) % 255)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	
	return img
}

func main() {
	// Create clean files without EXIF for filename pattern testing
	img := createTestImage(400, 300)
	
	files := []string{
		"messages/signal_20240315_143022.jpg",
		"messages/IMG-20240315-WA0001.jpg", 
		"messages/VID-20240315-WA0001.jpg",
		"messages/telegram_2024-03-15.jpg",
		"messages/InShot_20240315_143022.jpg",
		"messages/instagram_20240315_143022.jpg",
		"messages/SIGNAL_20240320_120000.JPG", // Case test
	}
	
	for _, filename := range files {
		file, err := os.Create(filename)
		if err != nil {
			fmt.Printf("Error creating %s: %v\n", filename, err)
			continue
		}
		
		// Save without EXIF metadata
		options := &jpeg.Options{Quality: 85}
		if err := jpeg.Encode(file, img, options); err != nil {
			fmt.Printf("Error encoding %s: %v\n", filename, err)
		} else {
			fmt.Printf("Created clean file: %s\n", filename)
		}
		file.Close()
	}
	
	fmt.Println("\nClean test files created without EXIF metadata!")
	fmt.Println("Now filename parsing should work correctly.")
}