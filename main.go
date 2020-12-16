package main

import "compress/gzip"
import "encoding/csv"
import "fmt"
import "image"
import "image/color"
import "image/draw"
import "io"
import "io/ioutil"
import "math"
import "path/filepath"
import "os"
import "strings"

import "github.com/jessevdk/go-flags"
import "github.com/llgcode/draw2d/draw2dimg"
import "github.com/tkrajina/gpxgo/gpx"
import "github.com/tormoder/fit"
import "github.com/codingsince1985/geo-golang/openstreetmap"

type Point struct {
	x, y float64
}

func readGPX(file string) []Point {
	points := []Point{}
	data, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Println("Unable to open", file, "continuing anyway.")
		return points
	}

	gpxFile, err := gpx.ParseBytes(data)
	if err != nil {
		fmt.Println("Unable to parse", file, "continuing anyway.")
		return points
	}

	for _, track := range gpxFile.Tracks {
		for _, segment := range track.Segments {
			for _, point := range segment.Points {
				points = append(
					points,
					Point{point.Longitude, point.Latitude},
				)
			}
		}
	}
	return points
}

func readFit(file string) []Point {
	points := []Point{}

	fi, err := os.Open(file)
	defer fi.Close()
	if err != nil {
		fmt.Println("Unable to open", file, "continuing anyway.")
		return points
	}

	data, err := gzip.NewReader(fi)
	defer data.Close()
	if err != nil {
		fmt.Println("Unable to unzip", file, "continuing anyway.")
		return points
	}

	fitFile, err := fit.Decode(data)
	if err != nil {
		fmt.Println("Unable to parse", file, "continuing anyway.")
		return points
	}

	activity, err := fitFile.Activity()
	if err != nil {
		fmt.Println("Unable to find activity in", file, "continuing anyway.")
		return points
	}

	for _, p := range activity.Records {
		points = append(
			points,
			Point{p.PositionLong.Degrees(), p.PositionLat.Degrees()},
		)
	}
	return points
}

type T func(float64) float64

func removeNaN(points []Point) []Point {
	valid := []Point{}
	for _, p := range points {
		if math.IsNaN(p.x) || math.IsNaN(p.y) {
			continue
		}
		valid = append(valid, p)
	}
	return valid
}

func addLine(image *image.RGBA, points []Point, tx T, ty T, color color.RGBA) {
	gc := draw2dimg.NewGraphicContext(image)

	gc.SetStrokeColor(color)
	gc.SetLineWidth(1)
	gc.BeginPath()
	first := points[0]
	gc.MoveTo(tx(first.x), ty(first.y))
	for i, p := range points {
		if i > 0 {
			gc.LineTo(tx(p.x), ty(p.y))
		}
	}
	gc.Stroke()
}

func transformer(mi float64, ma float64, size int, flip bool) T {
	s := float64(size)
	d := ma - mi
	scale := s / d
	inner := func(p float64) float64 {
		if flip {
			return s - (p-mi)*scale
		} else {
			return (p - mi) * scale
		}
	}
	return inner
}

func createImage(width int, height int, background color.RGBA) *image.RGBA {
	rect := image.Rect(0, 0, width, height)
	img := image.NewRGBA(rect)
	draw.Draw(img, img.Bounds(), &image.Uniform{background}, image.ZP, draw.Src)
	return img
}

type Config struct {
	width, height          int
	miny, maxy, minx, maxx float64
	activities, inDir, out string
	verbose                bool
}

func readConfig() Config {
	var opts struct {
		Height  int     `long:"height" description:"Height of output image" default:"500"`
		Width   int     `long:"width" description:"Width of output image" default:"1000"`
		Center  string  `short:"c" long:"center" description:"Location to center the map around" required:"true"`
		InDir   string  `short:"i" long:"in-dir" description:"Directory where the exported data is" required:"true"`
		Out     string  `short:"o" long:"out-file" description:"Output filename" required:"true"`
		Scale   float64 `short:"s" long:"scale" description:"Scale of map (bigger number shows more)" default:"1"`
		Verbose bool    `short:"v" long:"verbose" description:"Show more detailed output"`
	}
	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("Looking up \"%s\"...", opts.Center)
	geocoder := openstreetmap.Geocoder()
	location, _ := geocoder.Geocode(opts.Center)

	if location == nil {
		fmt.Println("failed!")
		fmt.Printf("Unable to find coordinates for center \"%s\".\n", opts.Center)
		os.Exit(1)
	}

	fmt.Println("found at", location.Lat, location.Lng)
	yRange := 0.06 * opts.Scale
	xRange := (float64(opts.Width) / float64(opts.Height)) * 0.2 * opts.Scale / 2

	cfg := Config{
		miny:    location.Lat - yRange,
		maxy:    location.Lat + yRange,
		minx:    location.Lng - xRange,
		maxx:    location.Lng + xRange,
		width:   opts.Width,
		height:  opts.Height,
		inDir:   opts.InDir,
		out:     opts.Out,
		verbose: opts.Verbose,
	}
	return cfg
}

type Activity struct {
	points         []Point
	kind, fileName string
}

func indices(line []string) (int, int) {
	kindIndex := 0
	fileIndex := 0
	for i, v := range line {
		switch v {
		case "Activity Type":
			kindIndex = i
		case "Filename":
			fileIndex = i
		}
	}
	return kindIndex, fileIndex
}

func getActivities(directory string) []Activity {
	activities := []Activity{}
	file := filepath.Join(directory, "activities.csv")

	csvFile, err := os.Open(file)
	defer csvFile.Close()
	if err != nil {
		fmt.Println("Unable to open", file, "continuing anyway.")
		return activities
	}

	r := csv.NewReader(csvFile)
	line, err := r.Read()
	if err != nil {
		fmt.Println("Unable to read from", file, "continuing anyway.")
		return activities
	}

	kindIndex, fileIndex := indices(line)

	fmt.Print("Parsing activities")
	for {
		line, err := r.Read()
		if err == io.EOF {
			break
		}
		fileName := line[fileIndex]
		kind := line[kindIndex]
		file := filepath.Join(directory, fileName)
		points := []Point{}
		if strings.HasSuffix(file, ".gpx") {
			points = readGPX(file)
		} else if strings.HasSuffix(file, ".fit.gz") {
			points = readFit(file)
		}
		points = removeNaN(points)
		if len(points) > 1 {
			activities = append(activities, Activity{
				points,
				kind,
				fileName,
			})
		}
		fmt.Print(".")
	}
	fmt.Println("done.")
	return activities
}

func main() {
	cfg := readConfig()
	image := createImage(cfg.width, cfg.height, color.RGBA{0, 0, 0, 255})

	activities := getActivities(cfg.inDir)

	ty := transformer(cfg.miny, cfg.maxy, cfg.height, true)
	tx := transformer(cfg.minx, cfg.maxx, cfg.width, false)

	colors := map[string]color.RGBA{
		"Ride":            color.RGBA{255, 255, 255, 255},
		"Run":             color.RGBA{200, 255, 255, 255},
		"Workout":         color.RGBA{200, 255, 255, 255},
		"Kayaking":        color.RGBA{255, 255, 200, 255},
		"Canoe":           color.RGBA{255, 255, 200, 255},
		"Swim":            color.RGBA{255, 255, 200, 255},
		"Nordic Ski":      color.RGBA{200, 200, 255, 255},
		"Backcountry Ski": color.RGBA{200, 200, 255, 255},
		"Ice Skate":       color.RGBA{200, 200, 255, 255},
		"Walk":            color.RGBA{255, 200, 200, 255},
		"Hike":            color.RGBA{255, 200, 200, 255},
	}

	unmatched := make(map[string]int)
	fmt.Printf("Generating %dx%d image", cfg.width, cfg.height)
	for _, act := range activities {
		color, ok := colors[act.kind]
		if ok {
			if cfg.verbose {
				fmt.Println("Drawing", act.fileName)
			}
			addLine(image, act.points, tx, ty, color)
			if cfg.verbose {
				fmt.Println("done")
			} else {
				fmt.Print(".")
			}
		} else {
			fmt.Print("U")
			unmatched[act.kind]++
		}
	}
	fmt.Println("done.")
	if len(unmatched) > 0 {
		fmt.Println("Found unmatched activities:")
		for k, v := range unmatched {
			fmt.Println(v, k)
		}
		fmt.Println("These will not be in the image.")
	}

	fmt.Printf("Exporting image as %s...", cfg.out)
	draw2dimg.SaveToPngFile(cfg.out, image)
	fmt.Println("done.")
}
