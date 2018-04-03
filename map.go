package main

import (
	"bytes"
	"fmt"
	"image/color"
	"io"

	staticMap "github.com/Luzifer/go-staticmaps"
	"github.com/fogleman/gg"
	"github.com/golang/geo/s2"
)

var markerColors = map[string]color.Color{
	"black":  color.RGBA{R: 145, G: 145, B: 145, A: 0xff},
	"brown":  color.RGBA{R: 178, G: 154, B: 123, A: 0xff},
	"green":  color.RGBA{R: 168, G: 196, B: 68, A: 0xff},
	"purple": color.RGBA{R: 177, G: 150, B: 191, A: 0xff},
	"yellow": color.RGBA{R: 237, G: 201, B: 107, A: 0xff},
	"blue":   color.RGBA{R: 163, G: 196, B: 253, A: 0xff},
	"gray":   color.RGBA{R: 204, G: 204, B: 204, A: 0xff},
	"orange": color.RGBA{R: 229, G: 165, B: 68, A: 0xff},
	"red":    color.RGBA{R: 246, G: 118, B: 112, A: 0xff},
	"white":  color.RGBA{R: 245, G: 244, B: 241, A: 0xff},
}

type markerSize float64

var markerSizes = map[string]markerSize{
	"tiny":  10,
	"mid":   15,
	"small": 20,
}

type marker struct {
	pos   s2.LatLng
	color color.Color
	size  markerSize
}

func (m marker) String() string {
	r, g, b, a := m.color.RGBA()
	return fmt.Sprintf("%s|%.0f|%d,%d,%d,%d", m.pos.String(), m.size, r, g, b, a)
}

func generateMap(center s2.LatLng, zoom int, marker []marker, x, y int, disableAttribution bool) (io.Reader, error) {
	ctx := staticMap.NewContext()
	ctx.SetUserAgent(fmt.Sprintf("Mozilla/5.0+(compatible; staticmap/%s; https://github.com/Luzifer/staticmap)", version))

	ctx.SetSize(x, y)
	ctx.SetCenter(center)
	ctx.SetZoom(zoom)

	if disableAttribution {
		ctx.ForceNoAttribution()
	}

	if marker != nil {
		for _, m := range marker {
			ctx.AddMarker(staticMap.NewMarker(m.pos, m.color, float64(m.size)))
		}
	}

	img, err := ctx.Render()
	if err != nil {
		return nil, err
	}

	pngCtx := gg.NewContextForImage(img)
	pngBuf := new(bytes.Buffer)
	return pngBuf, pngCtx.EncodePNG(pngBuf)
}
