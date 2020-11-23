package main

import (
	"flag"
	"github.com/golang/glog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	pl "github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
)

func getAlbumIndex(s *pl.Service, maxAlbums int) map[string]*[]string {
	glog.V(1).Info("Getting album index")

	as, err := getAlbums(s, nil, "")
	if err != nil {
		glog.Fatalf("Unable to call list: %v", err)
	}

	var wg sync.WaitGroup
	out := make(map[string]*[]string)

	for i, a := range as {
		if maxAlbums > 0 && i > maxAlbums {
			glog.Warningf("Reached max album count %d", maxAlbums)
			break
		}
		if _, ok := out[a.Title]; ok {
			// TODO: handle duplicate album titles
			glog.Warningf("Skipping duplicate album %s", a.Title)
			continue
		}

		wg.Add(1)
		fns := []string{}
		go func(a albumKey, fns *[]string) {
			ifns, err := getImagesForAlbum(s, a, nil, "")
			if err == nil {
				*fns = ifns
			} else {
				glog.Fatalf("Unable to call image search: %v", err)
			}
			wg.Done()
		}(a, &fns)
		out[a.Title] = &fns
	}

	wg.Wait()
	return out
}

type downloadedImages struct {
	pathIndex          map[string]string
	duplicateFilenames map[string]bool
}

func getDownloadedImages(root string) *downloadedImages {
	out := &downloadedImages{
		pathIndex:          make(map[string]string),
		duplicateFilenames: make(map[string]bool),
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		fn := filepath.Base(path)
		if _, ok := out.pathIndex[fn]; ok {
			out.duplicateFilenames[fn] = true
			return nil
		}
		out.pathIndex[fn] = path
		return nil
	})
	if err != nil {
		glog.Fatalf("Filed scanning paths: %v", err)
	}

	return out
}

// getDirectoryIndex returns a map[directory]map[filename]bool
func getDirectoryIndex(root string) map[string]map[string]bool {
	glog.V(1).Infof("Scanning local index")

	out := make(map[string]map[string]bool)

	albumDir := filepath.Join(root, "albums")

	err := filepath.Walk(albumDir, func(path string, info os.FileInfo, err error) error {
		if path == albumDir || strings.HasPrefix(filepath.Base(path), ".") {
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
		glog.Fatalf("Filed scanning paths: %v", err)
	}

	return out
}

func getOps(s *pl.Service, indexDirectory string, imageDirectory string, maxAlbums int) []operation {
	ibd, err := NewImagesByDate(s, -1)
	if err != nil {
		glog.Fatalf("Error getting images by date: %v", err)
	}
	ibd.VisitYears(func(y int) {
		ibd.VisitMonths(y, func(y, m int) {
			ibd.VisitDays(y, m, func(y, m, d int, fns []string) {
				glog.Infof("DAYS: %d-%d-%d %d images", y, m, d, len(fns))
			})
		})
	})

	di := getDirectoryIndex(indexDirectory)
	ips := getDownloadedImages(imageDirectory).pathIndex

	// TODO: support date index (i.e., year or month)

	var ops []operation
	if op := maybeCreateRootAlbumDir(indexDirectory); op != nil {
		ops = append(ops, *op)
	}

	for at, ifns := range getAlbumIndex(s, maxAlbums) {
		lns, ok := di[at]
		if !ok {
			// Need directory for the album
			ops = append(ops, createAlbumDirectory{
				albumTitle: at,
			})
			lns = make(map[string]bool)
		}
		mis := 0
		for _, ifn := range *ifns {
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
				glog.V(2).Infof("Missing %s for album %s", ifn, at)
			}
		}
		if mis > 0 {
			glog.Infof("Missing %d images for album %s", mis, at)
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

var (
	indexDirFlag  = flag.String("indexdir", "", "where the index will be created")
	imageDirFlag  = flag.String("imagedir", "", "where the images are located")
	tokenPathFlag = flag.String("tokenpath", "token.json", "path to the OAth token")
	maxAlbumsFlag = flag.Int("maxalbums", -1, "max number of albums to index")
)

func main() {
	flag.Parse()
	if *indexDirFlag == "" {
		glog.Fatalf("Please specify the index directory via --indexdir")
	}
	if *imageDirFlag == "" {
		glog.Fatalf("Please specify the image directory via --imagedir")
	}

	glog.Info("Starting indexer")

	client := getClient(pl.PhotoslibraryReadonlyScope, *tokenPathFlag)
	s, err := pl.New(client)
	if err != nil {
		glog.Fatalf("Unable to create pl Client %v", err)
	} else {
		glog.Info("Established Photos API client.")
	}

	os := getOps(s, *indexDirFlag, *imageDirFlag, *maxAlbumsFlag)
	glog.Infof("Running %d ops", len(os))
	for _, o := range os {
		glog.V(2).Infof("OP: %s", o.log())
		if err := o.run(*indexDirFlag); err != nil {
			glog.Errorf("Failed running: %s", o.log())
		}
	}

	glog.Info("Done")
}
