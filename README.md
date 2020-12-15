# bikemap
CLI application that renders a map of Strava activities. It uses openstreetmap to figure out where to center the map from a user-supplied location.

<img src="https://raw.githubusercontent.com/pjot/bikemap/main/static/example.png?raw=true" width="400" />

## Installation
Download the latest binary from [Releases](https://github.com/pjot/bikemap/releases) or build yourself: `go get && go build`

## Usage
### 1. Create a Download Request from Strava
* Settings > My Account > "Download or Delete Your Account" [Get Started] > #2
* You'll end up on https://www.strava.com/athlete/delete_your_account
* Request Your Archive
* You will get an export of all your Strava data in your email. This might take a while and is only possible to do once a week.
* Unzip the file you get in your email
### 2. Run the program!
* `./bikemap -i ~/my-export-dir -o file.png -c Stockholm`
* 3 arguments are required, `--in-dir/-i`, `--out-file/-o`, `--center/-c`.

### All command line arguments
```$ ./bikemap -h
Usage:
  bikemap [OPTIONS]

Application Options:
      --height=   Height of output image (default: 500)
      --width=    Width of output image (default: 1000)
  -c, --center=   Location to center the map around
  -i, --in-dir=   Directory where the exported data is
  -o, --out-file= Output filename
  -s, --scale=    Scale of map (bigger number shows more) (default: 1)

Help Options:
  -h, --help      Show this help message

