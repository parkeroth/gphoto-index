package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	pl "github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
)

const imageDirectory string = "/Users/parkeroth/Desktop/testdata"
const indexDirectory string = "/Users/parkeroth/Desktop/testout"

func getAlbumIndex(s *pl.Service) map[string]map[string]bool {
	log.Print("Getting album index")

	as, err := getAlbums(s, nil, "")
	if err != nil {
		log.Fatalf("Unable to call list: %v", err)
	}

	out := make(map[string]map[string]bool)

	// TODO: make this multi threaded

	ats := make(map[string]bool)

	for _, a := range as {
		if _, ok := ats[a.Title]; ok {
			// TODO: handle duplicate album titles
			log.Printf("WARNING: skipping duplicate album %s", a.Title)
			continue
		}
		ats[a.Title] = true

		ifns, err := getImageFilenames(s, a, nil, "")
		if err != nil {
			log.Fatalf("Unable to call image search: %v", err)
		}
		for _, fn := range ifns {
			as, ok := out[fn]
			if !ok {
				as = make(map[string]bool)
				out[fn] = as
			}
			out[fn][a.Title] = true
		}
	}

	return out
}

func getImagePaths(root string) []string {
	log.Print("Getting image paths")

	out := []string{}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		out = append(out, path)
		return nil
	})
	if err != nil {
		log.Fatalf("Filed scanning paths: %v", err)
	}

	return out
}

func getDirectoryIndex(root string) map[string]map[string]bool {
	log.Printf("Scanning local index")

	out := make(map[string]map[string]bool)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if path == root || strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}
		if info.IsDir() {
			out[filepath.Base(path)] = make(map[string]bool)
		} else {
			dir := filepath.Base(filepath.Dir(path))
			out[dir][filepath.Base(path)] = true
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Filed scanning paths: %v", err)
	}

	return out
}

func getOps(s *pl.Service) []operation {
	di := getDirectoryIndex(indexDirectory)
	ips := getImagePaths(imageDirectory)
	ai := getAlbumIndex(s)

	var ops []operation
	for _, ip := range ips {
		fn := filepath.Base(ip)
		if as, ok := ai[fn]; ok && len(as) > 0 {
			// Local image is in an album
			for a, _ := range as {
				is, ok := di[a]
				if !ok {
					// Need directory for the album
					ops = append(ops, createAlbumDirectory{
						albumTitle: a,
					})
					// Add fake entry to index to prevent duplicate op
					di[a] = make(map[string]bool)
				}
				if !ok || !is[fn] {
					// We either didn't have a directory or there wasn't a link
					// Either way we need to add one
					ops = append(ops, addAlbumLink{
						albumTitle: a,
						imagePath:  ip,
						filename:   fn,
					})
				}
				if ok {
					delete(is, fn)
					if len(is) == 0 {
						delete(di, a)
					}
				}
				delete(as, a)
			}
			if len(as) == 0 {
				delete(ai, fn)
			}
		}
		// TODO: support date index (i.e., year or month)
	}

	for a, is := range di {
		for i, _ := range is {
			ops = append(ops, removeAlbumLink{
				albumTitle: a,
				filename:   filepath.Base(i),
			})
		}
		ops = append(ops, removeAlbumDirectory{
			albumTitle: a,
		})
	}

	mis := 0
	for _, as := range ai {
		mis += len(as)
	}
	if mis > 0 {
		log.Printf("Missing %d images", mis)
	}

	return ops
}

func main() {
	log.Print("Starting indexer")

	client := getClient(pl.PhotoslibraryReadonlyScope)
	s, err := pl.New(client)
	if err != nil {
		log.Fatalf("Unable to create pl Client %v", err)
	} else {
		log.Print("Established Photos API client.")
	}

	for _, o := range getOps(s) {
		log.Printf("OP: %s", o.log())
		if err := o.run(indexDirectory); err != nil {
			log.Printf("Failed running: %s", o.log())
		}
	}
}
