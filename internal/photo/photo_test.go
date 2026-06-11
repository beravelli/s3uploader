package photo

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"regexp"
	"testing"
)

func loadSample(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/gps-sample.jpg")
	if err != nil {
		t.Fatalf("reading sample image: %v", err)
	}
	return data
}

func TestExtractEXIF(t *testing.T) {
	d, err := ExtractEXIF(bytes.NewReader(loadSample(t)))
	if err != nil {
		t.Fatalf("ExtractEXIF: %v", err)
	}

	if d.CameraModel != "COOLPIX P6000" {
		t.Errorf("camera model = %q, want COOLPIX P6000", d.CameraModel)
	}
	if d.Aperture != "f/5.9" {
		t.Errorf("aperture = %q, want f/5.9", d.Aperture)
	}
	if d.ShutterSpeed != "1/75s" {
		t.Errorf("shutter speed = %q, want 1/75s", d.ShutterSpeed)
	}
	if d.ISO != 64 {
		t.Errorf("ISO = %d, want 64", d.ISO)
	}
	if d.GPSLat == nil || d.GPSLng == nil {
		t.Fatal("expected GPS coordinates, got nil")
	}
	if math.Abs(*d.GPSLat-43.467448) > 0.001 || math.Abs(*d.GPSLng-11.885127) > 0.001 {
		t.Errorf("GPS = (%f, %f), want (~43.4674, ~11.8851)", *d.GPSLat, *d.GPSLng)
	}
}

func TestExtractEXIFNoData(t *testing.T) {
	// A bare PNG has no EXIF block; this must not be an error.
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	d, err := ExtractEXIF(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ExtractEXIF on PNG: %v", err)
	}
	if d.CameraModel != "" || d.GPSLat != nil {
		t.Errorf("expected empty EXIF data, got %+v", d)
	}
}

func TestExtractPalette(t *testing.T) {
	colors, err := ExtractPalette(bytes.NewReader(loadSample(t)), 5)
	if err != nil {
		t.Fatalf("ExtractPalette: %v", err)
	}
	if len(colors) != 5 {
		t.Fatalf("got %d colors, want 5", len(colors))
	}

	hexRe := regexp.MustCompile(`^#[0-9a-f]{6}$`)
	for _, c := range colors {
		if !hexRe.MatchString(c.Hex) {
			t.Errorf("invalid hex color %q", c.Hex)
		}
	}
}

func TestExtractPaletteSolidColor(t *testing.T) {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	colors, err := ExtractPalette(bytes.NewReader(buf.Bytes()), 5)
	if err != nil {
		t.Fatalf("ExtractPalette: %v", err)
	}
	if len(colors) == 0 {
		t.Fatal("expected at least one color")
	}
	// All buckets of a solid image must average to the same color.
	for _, c := range colors {
		if c.Hex != colors[0].Hex {
			t.Errorf("solid image produced differing colors: %v", colors)
			break
		}
	}
}

func TestExtractPaletteNotAnImage(t *testing.T) {
	if _, err := ExtractPalette(bytes.NewReader([]byte("not an image")), 5); err == nil {
		t.Error("expected error for non-image input")
	}
}
