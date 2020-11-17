module github.com/parkeroth/gphoto-index

go 1.13

require (
	github.com/gphotosuploader/googlemirror v0.0.0-00010101000000-000000000000
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	golang.org/x/oauth2 v0.0.0-20201109201403-9fd604954f58
)

replace github.com/gphotosuploader/googlemirror => github.com/parkeroth/googlemirror v0.4.1-0.20201116123350-0df7c95bce8d
