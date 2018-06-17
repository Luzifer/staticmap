package main

import (
	"crypto/sha256"
	"fmt"
	"strings"

	staticMap "github.com/Luzifer/go-staticmaps"
	"github.com/golang/geo/s2"
)

type postMapEnvelope struct {
	Center             postMapPoint   `json:"center"`
	Zoom               int            `json:"zoom"`
	Markers            postMapMarkers `json:"markers"`
	Width              int            `json:"width"`
	Height             int            `json:"height"`
	DisableAttribution bool           `json:"disable_attribution"`
	Overlays           postMapOverlay `json:"overlays"`
}

func (p postMapEnvelope) toGenerateMapConfig() (generateMapConfig, error) {
	result := generateMapConfig{
		Center:             p.Center.getPoint(),
		Zoom:               p.Zoom,
		Width:              p.Width,
		Height:             p.Height,
		DisableAttribution: p.DisableAttribution,
	}

	if p.Width > mapMaxX || p.Height > mapMaxY {
		return generateMapConfig{}, fmt.Errorf("Map size exceeds allowed bounds of %dx%d", mapMaxX, mapMaxY)
	}

	var err error
	if result.Markers, err = p.Markers.toMarkers(); err != nil {
		return generateMapConfig{}, err
	}

	if result.Overlays, err = p.Overlays.toOverlays(); err != nil {
		return generateMapConfig{}, err
	}

	return result, nil
}

type postMapMarker struct {
	Size  string       `json:"size"`
	Color string       `json:"color"`
	Coord postMapPoint `json:"coord"`
}

func (p postMapMarker) String() string {
	parts := []string{}

	if p.Size != "" {
		parts = append(parts, fmt.Sprintf("size:%s", p.Size))
	}

	if p.Color != "" {
		parts = append(parts, fmt.Sprintf("color:%s", p.Color))
	}

	parts = append(parts, p.Coord.String())
	return strings.Join(parts, "|")
}

type postMapMarkers []postMapMarker

func (p postMapMarkers) toMarkers() ([]marker, error) {
	raw := []string{}
	for _, pm := range p {
		raw = append(raw, pm.String())
	}

	return parseMarkerLocations(raw)
}

type postMapPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

func (p postMapPoint) String() string {
	return fmt.Sprintf("%f,%f", p.Lat, p.Lon)
}

func (p postMapPoint) getPoint() s2.LatLng {
	return s2.LatLngFromDegrees(p.Lat, p.Lon)
}

type postMapOverlay []string

func (p postMapOverlay) toOverlays() ([]*staticMap.TileProvider, error) {
	result := []*staticMap.TileProvider{}
	for _, pat := range p {

		for _, v := range []string{`{0}`, `{1}`, `{2}`} {
			if !strings.Contains(pat, v) {
				return nil, fmt.Errorf("Placeholder %q not found in pattern %q", v, pat)
			}
		}

		pat = strings.NewReplacer(`{0}`, `%[2]d`, `{1}`, `%[3]d`, `{2}`, `%[4]d`).Replace(pat)

		result = append(result, &staticMap.TileProvider{
			Name:       fmt.Sprintf("%x", sha256.Sum256([]byte(pat))),
			TileSize:   256,
			URLPattern: pat,
		})
	}

	return result, nil
}
