package photo

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

type EXIFData struct {
	CameraModel  string   `json:"camera_model,omitempty"`
	Aperture     string   `json:"aperture,omitempty"`
	ShutterSpeed string   `json:"shutter_speed,omitempty"`
	ISO          int      `json:"iso,omitempty"`
	FocalLength  string   `json:"focal_length,omitempty"`
	DateTaken    string   `json:"date_taken,omitempty"`
	GPSLat       *float64 `json:"gps_lat,omitempty"`
	GPSLng       *float64 `json:"gps_lng,omitempty"`
}

// ExtractEXIF parses EXIF data from r. A file with no EXIF block is not an
// error: it returns an empty EXIFData and nil error.
func ExtractEXIF(r io.Reader) (*EXIFData, error) {
	x, err := exif.Decode(r)
	if err != nil {
		return &EXIFData{}, nil
	}

	d := &EXIFData{}

	if tag, err := x.Get(exif.Model); err == nil {
		if s, err := tag.StringVal(); err == nil {
			d.CameraModel = strings.TrimSpace(strings.Trim(s, "\x00"))
		}
	}

	if tag, err := x.Get(exif.FNumber); err == nil {
		if num, den, err := tag.Rat2(0); err == nil && den != 0 {
			d.Aperture = fmt.Sprintf("f/%.1f", float64(num)/float64(den))
		}
	}

	if tag, err := x.Get(exif.ExposureTime); err == nil {
		if num, den, err := tag.Rat2(0); err == nil && num != 0 && den != 0 {
			sec := float64(num) / float64(den)
			if sec >= 1 {
				d.ShutterSpeed = fmt.Sprintf("%.4gs", sec)
			} else {
				d.ShutterSpeed = fmt.Sprintf("1/%.0fs", float64(den)/float64(num))
			}
		}
	}

	if tag, err := x.Get(exif.ISOSpeedRatings); err == nil {
		if iso, err := tag.Int(0); err == nil {
			d.ISO = iso
		}
	}

	if tag, err := x.Get(exif.FocalLength); err == nil {
		if num, den, err := tag.Rat2(0); err == nil && den != 0 {
			d.FocalLength = fmt.Sprintf("%.4gmm", float64(num)/float64(den))
		}
	}

	if dt, err := x.DateTime(); err == nil {
		d.DateTaken = dt.Format(time.RFC3339)
	}

	if lat, lng, err := x.LatLong(); err == nil {
		d.GPSLat = &lat
		d.GPSLng = &lng
	}

	return d, nil
}
