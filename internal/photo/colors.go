package photo

import (
	"fmt"
	"image"
	"io"
	"sort"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
)

type Color struct {
	Hex string `json:"hex"`
}

const sampleSize = 50

// ExtractPalette decodes the image, downsamples it to 50x50, and runs
// median-cut over the pixels to find the n most dominant colors.
func ExtractPalette(r io.Reader, n int) ([]Color, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	dst := image.NewRGBA(image.Rect(0, 0, sampleSize, sampleSize))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

	pixels := make([][3]uint8, 0, sampleSize*sampleSize)
	for y := 0; y < sampleSize; y++ {
		for x := 0; x < sampleSize; x++ {
			pr, pg, pb, pa := dst.At(x, y).RGBA()
			if pa < 1<<15 {
				continue
			}
			pixels = append(pixels, [3]uint8{uint8(pr >> 8), uint8(pg >> 8), uint8(pb >> 8)})
		}
	}
	if len(pixels) == 0 {
		return []Color{}, nil
	}

	buckets := medianCut(pixels, n)

	colors := make([]Color, 0, len(buckets))
	type avgColor struct {
		r, g, b float64
		lum     float64
	}
	avgs := make([]avgColor, 0, len(buckets))
	for _, b := range buckets {
		var sr, sg, sb float64
		for _, p := range b {
			sr += float64(p[0])
			sg += float64(p[1])
			sb += float64(p[2])
		}
		n := float64(len(b))
		a := avgColor{r: sr / n, g: sg / n, b: sb / n}
		a.lum = 0.299*a.r + 0.587*a.g + 0.114*a.b
		avgs = append(avgs, a)
	}
	sort.Slice(avgs, func(i, j int) bool { return avgs[i].lum > avgs[j].lum })
	for _, a := range avgs {
		colors = append(colors, Color{Hex: fmt.Sprintf("#%02x%02x%02x", uint8(a.r), uint8(a.g), uint8(a.b))})
	}
	return colors, nil
}

// medianCut splits the pixel set into up to n buckets by repeatedly splitting
// the bucket with the widest channel range at its median.
func medianCut(pixels [][3]uint8, n int) [][][3]uint8 {
	buckets := [][][3]uint8{pixels}
	for len(buckets) < n {
		// Find the bucket with the largest channel range that can still split.
		bestIdx, bestRange, bestChan := -1, -1, 0
		for i, b := range buckets {
			if len(b) < 2 {
				continue
			}
			for ch := 0; ch < 3; ch++ {
				lo, hi := 255, 0
				for _, p := range b {
					v := int(p[ch])
					if v < lo {
						lo = v
					}
					if v > hi {
						hi = v
					}
				}
				if hi-lo > bestRange {
					bestRange, bestIdx, bestChan = hi-lo, i, ch
				}
			}
		}
		if bestIdx == -1 {
			break
		}
		b := buckets[bestIdx]
		ch := bestChan
		sort.Slice(b, func(i, j int) bool { return b[i][ch] < b[j][ch] })
		mid := len(b) / 2
		buckets[bestIdx] = b[:mid]
		buckets = append(buckets, b[mid:])
	}
	return buckets
}
