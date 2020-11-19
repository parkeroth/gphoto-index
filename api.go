package main

import (
	"github.com/golang/glog"

	pl "github.com/gphotosuploader/googlemirror/api/photoslibrary/v1"
)

type albumKey struct {
	Id    string
	Title string
}

func getAlbums(s *pl.Service, ks []albumKey, pt string) ([]albumKey, error) {
	alc := s.Albums.List().PageSize(50)
	if pt == "" {
		glog.V(2).Info("Sending API call: initial album list")
	} else {
		glog.V(2).Info("Sending API call: additional album list")
		alc = alc.PageToken(pt)
	}

	alr, err := alc.Do()
	if err != nil {
		return nil, err
	}
	for _, a := range alr.Albums {
		ks = append(ks, albumKey{
			Id:    a.Id,
			Title: a.Title,
		})
	}

	if alr.NextPageToken == "" {
		glog.V(1).Infof("Found %d albums", len(ks))
		return ks, nil
	}
	return getAlbums(s, ks, alr.NextPageToken)
}

func getImageFilenames(s *pl.Service, ak albumKey, fns []string, pt string) ([]string, error) {
	sreq := &pl.SearchMediaItemsRequest{
		AlbumId:  ak.Id,
		PageSize: 100,
	}
	if pt == "" {
		glog.V(2).Infof("Sending API call: initial image search for album: %s", ak.Title)
	} else {
		glog.V(2).Infof("Sending API call: additional image search for album: %s %s", ak.Title, pt[len(pt)-8:])
		sreq.PageToken = pt
	}

	sresp, err := s.MediaItems.Search(sreq).Do()
	if err != nil {
		return nil, err
	}
	for _, mi := range sresp.MediaItems {
		fns = append(fns, mi.Filename)
	}

	if sresp.NextPageToken == "" {
		glog.V(1).Infof("Found %d images for album %s", len(fns), ak.Title)
		return fns, nil
	}
	return getImageFilenames(s, ak, fns, sresp.NextPageToken)
}
