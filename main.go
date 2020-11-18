package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	pl "github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
)

const maxAlbums int = 6
const imageDirectory string = "/Users/parkeroth/Desktop/testdata"
const indexDirectory string = "/Users/parkeroth/Desktop/testout"

func getAlbumIndex(s *pl.Service) map[string][]string {
	log.Print("Getting album index")

	as, err := getAlbums(s, nil, "")
	if err != nil {
		log.Fatalf("Unable to call list: %v", err)
	}

	out := make(map[string][]string)

	// TODO: make this multi threaded

	for i, a := range as {
		if i > maxAlbums {
			log.Printf("WARNING: reached max album count %d", maxAlbums)
			break
		}
		if _, ok := out[a.Title]; ok {
			// TODO: handle duplicate album titles
			log.Printf("WARNING: skipping duplicate album %s", a.Title)
			continue
		}

		ifns, err := getImageFilenames(s, a, nil, "")
		if err == nil {
			out[a.Title] = ifns
		} else {
			log.Fatalf("Unable to call image search: %v", err)
		}
	}

	return out
}

func getImageIndex(root string) map[string]string {
	out := make(map[string]string)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		fn := filepath.Base(path)
		if _, ok := out[fn]; ok {
			// TODO: handle duplicate filenames
			log.Printf("WARNING: skipping duplicate image %s", fn)
			return nil
		}
		out[fn] = path
		return nil
	})
	if err != nil {
		log.Fatalf("Filed scanning paths: %v", err)
	}

	return out
}

// getDirectoryIndex returns a map[directory]map[filename]bool
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
	ips := getImageIndex(imageDirectory)

	// TODO: support date index (i.e., year or month)

	var ops []operation
	for at, ifns := range getAlbumIndex(s) {
		lns, ok := di[at]
		if !ok {
			// Need directory for the album
			ops = append(ops, createAlbumDirectory{
				albumTitle: at,
			})
			lns = make(map[string]bool)
		}
		mis := 0
		for _, ifn := range ifns {
			if _, ok := lns[ifn]; ok {
				// Already have symlink
				delete(lns, ifn)
				continue
			}
			if ip, ok := ips[ifn]; ok {
				ops = append(ops, addAlbumLink{
					albumTitle: at,
					imagePath:  ip,
					filename:   ifn,
				})
			} else {
				mis += 1
			}
		}
		if mis > 0 {
			log.Printf("Missing %d images for album %s", mis, at)
		}
		for ln, _ := range lns {
			ops = append(ops, removeAlbumLink{
				albumTitle: at,
				filename:   filepath.Base(ln),
			})
		}
		delete(di, at)
	}

	for at, lns := range di {
		for ln, _ := range lns {
			ops = append(ops, removeAlbumLink{
				albumTitle: at,
				filename:   filepath.Base(ln),
			})
		}
		ops = append(ops, removeAlbumDirectory{
			albumTitle: at,
		})
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
