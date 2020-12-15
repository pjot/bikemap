package main

import "compress/gzip"
import "encoding/csv"
import "fmt"
import "image"
import "image/color"
import "image/draw"
import "io"
import "io/ioutil"
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
	data, _ := ioutil.ReadFile(file)
	gpxFile, _ := gpx.ParseBytes(data)

	points := []Point{}
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
	fi, _ := os.Open(file)
	defer fi.Close()

	data, _ := gzip.NewReader(fi)
	defer data.Close()

	fitFile, _ := fit.Decode(data)
	activity, _ := fitFile.Activity()

	points := []Point{}
	for _, p := range activity.Records {
		points = append(
			points,
			Point{p.PositionLong.Degrees(), p.PositionLat.Degrees()},
		)
	}
	return points
}

type T func(float64) float64

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

func gpxPoints(activities string) [][]Point {
	gpxs, _ := filepath.Glob(activities + "*.gpx")
	fmt.Printf("Found %d .gpx files...", len(gpxs))

	tracks := [][]Point{}
	for i, file := range gpxs {
		if i%10 == 0 {
			fmt.Print(".")
		}
		points := readGPX(file)
		tracks = append(tracks, points)
	}

	fmt.Println("done.")
	return tracks
}

func fitPoints(activities string) [][]Point {
	fits, _ := filepath.Glob(activities + "*.fit.gz")
	fmt.Printf("Found %d .fit.gz files...", len(fits))

	tracks := [][]Point{}
	for i, file := range fits {
		if i%10 == 0 {
			fmt.Print(".")
		}
		points := readFit(file)
		if len(points) > 0 {
			tracks = append(tracks, points)
		}
	}

	fmt.Println("done.")
	return tracks
}

type Config struct {
	width, height          int
	miny, maxy, minx, maxx float64
	activities, inDir, out string
}

func readConfig() Config {
	var opts struct {
		Height int     `long:"height" description:"Height of output image" default:"500"`
		Width  int     `long:"width" description:"Width of output image" default:"1000"`
		Center string  `short:"c" long:"center" description:"Location to center the map around" required:"true"`
		InDir  string  `short:"i" long:"in-dir" description:"Directory where the exported data is" required:"true"`
		Out    string  `short:"o" long:"out-file" description:"Output filename" required:"true"`
		Scale  float64 `short:"s" long:"scale" description:"Scale of map (bigger number shows more)" default:"1"`
	}
	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
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
		miny:   location.Lat - yRange,
		maxy:   location.Lat + yRange,
		minx:   location.Lng - xRange,
		maxx:   location.Lng + xRange,
		width:  opts.Width,
		height: opts.Height,
		inDir:  opts.InDir,
		out:    opts.Out,
	}
	return cfg
}

type Activity struct {
	points []Point
	kind   string
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
	file := filepath.Join(directory, "activities.csv")

	csvFile, _ := os.Open(file)
	defer csvFile.Close()

	r := csv.NewReader(csvFile)
	line, _ := r.Read()
	kindIndex, fileIndex := indices(line)

	activities := []Activity{}
	fmt.Print("Parsing activities")
	for {
		line, err := r.Read()
		if err == io.EOF {
			break
		}
		file := filepath.Join(directory, line[fileIndex])
		kind := line[kindIndex]
		points := []Point{}
		if strings.HasSuffix(file, ".gpx") {
			points = readGPX(file)
		} else if strings.HasSuffix(file, ".fit.gz") {
			points = readFit(file)
		}
		if len(points) > 0 {
			activities = append(activities, Activity{
				points,
				kind,
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
		"Ride":       color.RGBA{255, 255, 255, 255},
		"Run":        color.RGBA{200, 255, 255, 255},
		"Kayaking":   color.RGBA{255, 255, 200, 255},
		"Nordic Ski": color.RGBA{200, 200, 255, 255},
		"Walk":       color.RGBA{255, 200, 200, 255},
	}

	unmatched := make(map[string]int)
	fmt.Printf("Generating %dx%d image", cfg.width, cfg.height)
	for _, act := range activities {
		color, ok := colors[act.kind]
		if ok {
			addLine(image, act.points, tx, ty, color)
			fmt.Print(".")
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
