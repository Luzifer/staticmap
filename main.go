package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	httpHelper "github.com/Luzifer/go_helpers/v2/http"
	"github.com/Luzifer/rconfig/v2"
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/golang/geo/s2"
	"github.com/gorilla/mux"
	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	cfg struct {
		CacheDir       string        `flag:"cache-dir" default:"cache" description:"Directory to save the cached images to"`
		ForceCache     time.Duration `flag:"force-cache" default:"24h" description:"Force map to be cached for this duration"`
		Listen         string        `flag:"listen" default:":3000" description:"IP/Port to listen on"`
		MaxSize        string        `flag:"max-size" default:"1024x1024" description:"Maximum map size requestable"`
		RateLimit      float64       `flag:"rate-limit" default:"1" description:"How many requests to allow per time"`
		RateLimitTime  time.Duration `flag:"rate-limit-time" default:"1s" description:"Time interval to allow N requests in"`
		VersionAndExit bool          `flag:"version" default:"false" description:"Print version information and exit"`
	}

	mapMaxX, mapMaxY int
	cacheFunc        cacheFunction = filesystemCache // For now this is simply set and might be extended later

	version = "dev"
)

func initApp() (err error) {
	rconfig.AutoEnv(true)
	if err = rconfig.ParseAndValidate(&cfg); err != nil {
		return errors.Wrap(err, "parsing CLI parameters")
	}

	if mapMaxX, mapMaxY, err = parseSize(cfg.MaxSize); err != nil {
		return errors.Wrap(err, "parsing max-size")
	}

	return nil
}

func main() {
	var err error
	if err = initApp(); err != nil {
		logrus.WithError(err).Fatal("initializing app")
	}

	if cfg.VersionAndExit {
		fmt.Printf("staticmap %s\n", version) //nolint:forbidigo
		return
	}

	rateLimit := tollbooth.NewLimiter(cfg.RateLimit, &limiter.ExpirableOptions{
		DefaultExpirationTTL: cfg.RateLimitTime,
	})
	rateLimit.SetIPLookups([]string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"})

	r := mux.NewRouter()
	r.HandleFunc("/status", func(res http.ResponseWriter, r *http.Request) { http.Error(res, "I'm fine", http.StatusOK) })
	r.Handle("/map.png", tollbooth.LimitFuncHandler(rateLimit, handleMapRequest)).Methods("GET")
	r.Handle("/map.png", tollbooth.LimitFuncHandler(rateLimit, handlePostMapRequest)).Methods("POST")

	server := &http.Server{
		Addr:              cfg.Listen,
		Handler:           httpHelper.NewHTTPLogHandlerWithLogger(r, logrus.StandardLogger()),
		ReadHeaderTimeout: time.Second,
	}

	logrus.WithField("version", version).Info("staticmap started")
	if err = server.ListenAndServe(); err != nil {
		logrus.WithError(err).Fatal("running HTTP server")
	}
}

func handleMapRequest(res http.ResponseWriter, r *http.Request) {
	var (
		err       error
		mapReader io.ReadCloser
		opts      = generateMapConfig{
			DisableAttribution: r.URL.Query().Get("no-attribution") == "true",
		}
	)

	if opts.Center, err = parseCoordinate(r.URL.Query().Get("center")); err != nil {
		http.Error(res, fmt.Sprintf("Unable to parse 'center' parameter: %s", err), http.StatusBadRequest)
		return
	}

	if opts.Zoom, err = strconv.Atoi(r.URL.Query().Get("zoom")); err != nil {
		http.Error(res, fmt.Sprintf("Unable to parse 'zoom' parameter: %s", err), http.StatusBadRequest)
		return
	}

	if opts.Width, opts.Height, err = parseSize(r.URL.Query().Get("size")); err != nil {
		http.Error(res, fmt.Sprintf("Unable to parse 'size' parameter: %s", err), http.StatusBadRequest)
		return
	}

	if opts.Markers, err = parseMarkerLocations(r.URL.Query()["markers"]); err != nil {
		http.Error(res, fmt.Sprintf("Unable to parse 'markers' parameter: %s", err), http.StatusBadRequest)
		return
	}

	if mapReader, err = cacheFunc(opts); err != nil {
		logrus.Errorf("map render failed: %s (Request: %s)", err, r.URL.String())
		http.Error(res, fmt.Sprintf("I experienced difficulties rendering your map: %s", err), http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := mapReader.Close(); err != nil {
			logrus.WithError(err).Error("closing map cache reader (leaked fd)")
		}
	}()

	res.Header().Set("Content-Type", "image/png")
	res.Header().Set("Cache-Control", "public")

	if _, err = io.Copy(res, mapReader); err != nil {
		logrus.WithError(err).Debug("writing image to HTTP client")
	}
}

func handlePostMapRequest(res http.ResponseWriter, r *http.Request) {
	var (
		body      = postMapEnvelope{}
		mapReader io.ReadCloser
	)

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(res, fmt.Sprintf("Unable to parse input: %s", err), http.StatusBadRequest)
		return
	}

	opts, err := body.toGenerateMapConfig()
	if err != nil {
		http.Error(res, fmt.Sprintf("Unable to process input: %s", err), http.StatusBadRequest)
		return
	}

	if mapReader, err = cacheFunc(opts); err != nil {
		logrus.Errorf("map render failed: %s (Request: %s)", err, r.URL.String())
		http.Error(res, fmt.Sprintf("I experienced difficulties rendering your map: %s", err), http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := mapReader.Close(); err != nil {
			logrus.WithError(err).Error("closing map cache reader (leaked fd)")
		}
	}()

	res.Header().Set("Content-Type", "image/png")
	res.Header().Set("Cache-Control", "public")

	if _, err = io.Copy(res, mapReader); err != nil {
		logrus.WithError(err).Debug("writing image to HTTP client")
	}
}

func parseCoordinate(coord string) (s2.LatLng, error) {
	if coord == "" {
		return s2.LatLng{}, errors.New("No coordinate given")
	}

	parts := strings.Split(coord, ",")
	if len(parts) != 2 { //nolint:gomnd
		return s2.LatLng{}, errors.New("Coordinate not in format lat,lon")
	}

	var (
		lat, lon float64
		err      error
	)

	if lat, err = strconv.ParseFloat(parts[0], 64); err != nil {
		return s2.LatLng{}, errors.New("Latitude not parseable as float")
	}

	if lon, err = strconv.ParseFloat(parts[1], 64); err != nil {
		return s2.LatLng{}, errors.New("Longitude not parseable as float")
	}

	pt := s2.LatLngFromDegrees(lat, lon)
	return pt, nil
}

func parseSize(size string) (x, y int, err error) {
	if size == "" {
		return 0, 0, errors.New("No size given")
	}

	parts := strings.Split(size, "x")
	if len(parts) != 2 { //nolint:gomnd
		return 0, 0, errors.New("Size not in format 600x300")
	}

	if x, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, errors.Wrap(err, "parsing width")
	}

	if y, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, errors.Wrap(err, "parsing height")
	}

	if (x > mapMaxX || y > mapMaxY) && mapMaxX > 0 && mapMaxY > 0 {
		return 0, 0, errors.Errorf("map size exceeds allowed bounds of %dx%d", mapMaxX, mapMaxY)
	}

	return x, y, nil
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
				s, ok := markerSizes[strings.TrimPrefix(p, "size:")]
				if !ok {
					return nil, errors.Errorf("bad marker size %q", strings.TrimPrefix(p, "size:"))
				}
				size = s

			case strings.HasPrefix(p, "color:0x"):
				c, err := colorful.Hex("#" + strings.TrimPrefix(p, "color:0x"))
				if err != nil {
					return nil, errors.Wrapf(err, "parsing color %q", strings.TrimPrefix(p, "color:"))
				}
				col = c

			case strings.HasPrefix(p, "color:"):
				c, ok := markerColors[strings.TrimPrefix(p, "color:")]
				if !ok {
					return nil, errors.Errorf("bad color name %q", strings.TrimPrefix(p, "color:"))
				}
				col = c

			default:
				pos, err := parseCoordinate(p)
				if err != nil {
					return nil, errors.Errorf("unparsable chunk found in marker: %q", p)
				}
				result = append(result, marker{
					pos:   pos,
					color: col,
					size:  size,
				})
			}
		}
	}

	return result, nil
}
