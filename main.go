package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	httpHelper "github.com/Luzifer/go_helpers/http"
	"github.com/Luzifer/rconfig"
	log "github.com/Sirupsen/logrus"
	"github.com/didip/tollbooth"
	"github.com/golang/geo/s2"
	"github.com/gorilla/mux"
	colorful "github.com/lucasb-eyer/go-colorful"
)

var (
	cfg struct {
		CacheDir       string        `flag:"cache-dir" default:"cache" env:"CACHE_DIR" description:"Directory to save the cached images to"`
		ForceCache     time.Duration `flag:"force-cache" default:"24h" env:"FORCE_CACHE" description:"Force map to be cached for this duration"`
		Listen         string        `flag:"listen" default:":3000" description:"IP/Port to listen on"`
		MaxSize        string        `flag:"max-size" default:"1024x1024" env:"MAX_SIZE" description:"Maximum map size requestable"`
		RateLimit      int64         `flag:"rate-limit" default:"1" env:"RATE_LIMIT" description:"How many requests to allow per time"`
		RateLimitTime  time.Duration `flag:"rate-limit-time" default:"1s" env:"RATE_LIMIT_TIME" description:"Time interval to allow N requests in"`
		VersionAndExit bool          `flag:"version" default:"false" description:"Print version information and exit"`
	}

	mapMaxX, mapMaxY int
	cacheFunc        cacheFunction = filesystemCache // For now this is simply set and might be extended later

	version = "dev"
)

func init() {
	var err error
	if err = rconfig.ParseAndValidate(&cfg); err != nil {
		log.Fatalf("Unable to parse CLI parameters")
	}

	if mapMaxX, mapMaxY, err = parseSize(cfg.MaxSize, false); err != nil {
		log.Fatalf("Unable to parse max-size: %s", err)
	}

	if cfg.VersionAndExit {
		fmt.Printf("staticmap %s\n", version)
	}
}

func main() {
	rateLimit := tollbooth.NewLimiter(cfg.RateLimit, cfg.RateLimitTime)
	rateLimit.IPLookups = []string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"}

	r := mux.NewRouter()
	r.HandleFunc("/status", func(res http.ResponseWriter, r *http.Request) { http.Error(res, "I'm fine", http.StatusOK) })
	r.Handle("/map.png", tollbooth.LimitFuncHandler(rateLimit, handleMapRequest))
	log.Fatalf("HTTP Server exitted: %s", http.ListenAndServe(cfg.Listen, httpHelper.NewHTTPLogHandler(r)))
}

func handleMapRequest(res http.ResponseWriter, r *http.Request) {
	var (
		center    *s2.LatLng
		err       error
		mapReader io.ReadCloser
		markers   []marker
		x, y      int
		zoom      int
	)

	if center, err = parseCoordinate(r.URL.Query().Get("center")); err != nil {
		http.Error(res, fmt.Sprintf("Unable to parse 'center' parameter: %s", err), http.StatusBadRequest)
		return
	}

	if zoom, err = strconv.Atoi(r.URL.Query().Get("zoom")); err != nil {
		http.Error(res, fmt.Sprintf("Unable to parse 'zoom' parameter: %s", err), http.StatusBadRequest)
		return
	}

	if x, y, err = parseSize(r.URL.Query().Get("size"), true); err != nil {
		http.Error(res, fmt.Sprintf("Unable to parse 'size' parameter: %s", err), http.StatusBadRequest)
		return
	}

	if markers, err = parseMarkerLocations(r.URL.Query()["markers"]); err != nil {
		http.Error(res, fmt.Sprintf("Unable to parse 'markers' parameter: %s", err), http.StatusBadRequest)
		return
	}

	if mapReader, err = cacheFunc(*center, zoom, markers, x, y); err != nil {
		log.Errorf("Map render failed: %s (Request: %s)", err, r.URL.String())
		http.Error(res, fmt.Sprintf("I experienced difficulties rendering your map: %s", err), http.StatusInternalServerError)
		return
	}
	defer mapReader.Close()

	res.Header().Set("Content-Type", "image/png")
	res.Header().Set("Cache-Control", "public")
	io.Copy(res, mapReader)
}

func parseCoordinate(coord string) (*s2.LatLng, error) {
	if coord == "" {
		return nil, errors.New("No coordinate given")
	}

	parts := strings.Split(coord, ",")
	if len(parts) != 2 {
		return nil, errors.New("Coordinate not in format lat,lon")
	}

	var (
		lat, lon float64
		err      error
	)

	if lat, err = strconv.ParseFloat(parts[0], 64); err != nil {
		return nil, errors.New("Latitude not parseable as float")
	}

	if lon, err = strconv.ParseFloat(parts[1], 64); err != nil {
		return nil, errors.New("Longitude not parseable as float")
	}

	pt := s2.LatLngFromDegrees(lat, lon)
	return &pt, nil
}

func parseSize(size string, validate bool) (x, y int, err error) {
	if size == "" {
		return 0, 0, errors.New("No size given")
	}

	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return 0, 0, errors.New("Size not in format 600x300")
	}

	if x, err = strconv.Atoi(parts[0]); err != nil {
		return
	}

	if y, err = strconv.Atoi(parts[1]); err != nil {
		return
	}

	if validate {
		if x > mapMaxX || y > mapMaxY {
			err = fmt.Errorf("Map size exceeds allowed bounds of %dx%d", mapMaxX, mapMaxY)
			return
		}
	}

	return
}

func parseMarkerLocations(markers []string) ([]marker, error) {
	if markers == nil {
		// No markers parameters passed, lets ignore this
		return nil, nil
	}

	result := []marker{}

	for _, markerInformation := range markers {
		parts := strings.Split(markerInformation, "|")

		var (
			size = markerSizes["small"]
			col  = markerColors["red"]
		)

		for _, p := range parts {
			switch {
			case strings.HasPrefix(p, "size:"):
				if s, ok := markerSizes[strings.TrimPrefix(p, "size:")]; ok {
					size = s
				} else {
					return nil, fmt.Errorf("Bad marker size %q", strings.TrimPrefix(p, "size:"))
				}
			case strings.HasPrefix(p, "color:0x"):
				if c, err := colorful.Hex("#" + strings.TrimPrefix(p, "color:0x")); err == nil {
					col = c
				} else {
					return nil, fmt.Errorf("Unable to parse color %q: %s", strings.TrimPrefix(p, "color:"), err)
				}
			case strings.HasPrefix(p, "color:"):
				if c, ok := markerColors[strings.TrimPrefix(p, "color:")]; ok {
					col = c
				} else {
					return nil, fmt.Errorf("Bad color name %q", strings.TrimPrefix(p, "color:"))
				}
			default:
				pos, err := parseCoordinate(p)
				if err != nil {
					return nil, fmt.Errorf("Unparsable chunk found in marker: %q", p)
				}
				result = append(result, marker{
					pos:   *pos,
					color: col,
					size:  size,
				})
			}
		}
	}

	return result, nil
}
