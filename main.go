package main

import "compress/gzip"

import "fmt"
import "image"
import "image/color"
import "image/draw"
import "io/ioutil"
import "path/filepath"
import "os"

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

func addLine(image *image.RGBA, points []Point, tx T, ty T) {
	gc := draw2dimg.NewGraphicContext(image)
	gc.SetStrokeColor(color.RGBA{255, 255, 255, 255})
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
	activities, out        string
}

func readConfig() Config {
	var opts struct {
		Height     int    `long:"height" description:"Height of output image" default:"500"`
		Width      int    `long:"width" description:"Width of output image" default:"1000"`
		Center     string `short:"c" long:"center" description:"Location to center the map around" required:"true"`
		Activities string `short:"i" long:"in-dir" description:"Directory where the activity fields are" required:"true"`
		Out        string `short:"o" long:"out-file" description:"Output filename" required:"true"`
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
	cfg := Config{
		miny:       location.Lat - 0.06,
		maxy:       location.Lat + 0.06,
		minx:       location.Lng - 0.2,
		maxx:       location.Lng + 0.2,
		width:      opts.Width,
		height:     opts.Height,
		activities: opts.Activities,
		out:        opts.Out,
	}
	return cfg
}

func main() {
	cfg := readConfig()
	image := createImage(cfg.width, cfg.height, color.RGBA{0, 0, 0, 255})

	gpx := gpxPoints(cfg.activities)
	fits := fitPoints(cfg.activities)
	tracks := append(gpx, fits...)

	ty := transformer(cfg.miny, cfg.maxy, cfg.height, true)
	tx := transformer(cfg.minx, cfg.maxx, cfg.width, false)

	fmt.Printf("Generating %dx%d image...", cfg.width, cfg.height)
	for _, points := range tracks {
		addLine(image, points, tx, ty)
	}
	fmt.Println("done.")

	fmt.Printf("Exporting image as %s...", cfg.out)
	draw2dimg.SaveToPngFile(cfg.out, image)
	fmt.Println("done.")
}
