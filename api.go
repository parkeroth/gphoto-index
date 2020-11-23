package main

import (
	"errors"
	"github.com/golang/glog"
	"sort"
	"time"

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

func getImagesForAlbum(s *pl.Service, ak albumKey, fns []string, pt string) ([]string, error) {
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
	return getImagesForAlbum(s, ak, fns, sresp.NextPageToken)
}

type dayIndex map[int][]string
type monthIndex map[int]dayIndex
type dateIndex map[int]monthIndex

type ImagesByDate struct {
	index      dateIndex
	imagesSeen map[string]bool
}

func NewImagesByDate(s *pl.Service, maxImages int) (*ImagesByDate, error) {
	ibd := &ImagesByDate{
		index:      make(dateIndex),
		imagesSeen: make(map[string]bool),
	}
	if err := fillImagesByDate(s, nil, ibd, "", maxImages); err != nil {
		return nil, err
	}
	return ibd, nil
}

func (ibd *ImagesByDate) VisitYears(f func(y int)) {
	sys := make([]int, len(ibd.index))
	i := 0
	for y := range ibd.index {
		sys[i] = y
		i++
	}
	sort.Ints(sys)
	for _, y := range sys {
		f(y)
	}
}

func (ibd *ImagesByDate) VisitMonths(y int, f func(y, m int)) {
	if mi, ok := ibd.index[y]; ok {
		glog.Infof("B")
		sms := make([]int, len(mi))
		i := 0
		for m := range mi {
			sms[i] = m
			i++
		}
		sort.Ints(sms)
		for _, m := range sms {
			f(y, m)
		}
	}
}

func (ibd *ImagesByDate) VisitDays(y, m int, f func(y, m, d int, filenames []string)) {
	if mi, ok := ibd.index[y]; ok {
		if di, ok := mi[m]; ok {
			sds := make([]int, len(di))
			i := 0
			for d := range di {
				sds[i] = d
				i++
			}
			sort.Ints(sds)
			for _, d := range sds {
				f(y, m, d, di[d])
			}
		}
	}
}

func (ibd *ImagesByDate) AddImage(filename, creationTime string) error {
	if _, ok := ibd.imagesSeen[filename]; ok {
		return errors.New("Duplicate image")
	}
	ibd.imagesSeen[filename] = true

	t, err := time.Parse(time.RFC3339, creationTime)
	if err != nil {
		return errors.New("Couldn't parse creation time")
	}

	mi, ok := ibd.index[int(t.Year())]
	if !ok {
		ibd.index[int(t.Year())] = make(monthIndex)
		mi, ok = ibd.index[int(t.Year())]
	}
	di, ok := mi[int(t.Month())]
	if !ok {
		mi[int(t.Month())] = make(dayIndex)
		di, ok = mi[int(t.Month())]
	}
	l, ok := di[int(t.Day())]
	if !ok {
		l = []string{}
	}
	di[int(t.Day())] = append(l, filename)
	glog.V(3).Infof("Adding %s %s", filename, creationTime)
	return nil
}

func (ibd *ImagesByDate) Size() int {
	return len(ibd.imagesSeen)
}

func getDate(t time.Time) *pl.Date {
	return &pl.Date{
		Day:   int64(t.Day()),
		Month: int64(t.Month()),
		Year:  int64(t.Year()),
	}
}

func fillImagesByDate(s *pl.Service, st *time.Time, ibd *ImagesByDate, pt string, max int) error {
	if max >= 0 && ibd.Size() >= max {
		glog.Warningf("Reached max image count: %d", max)
		return nil
	}

	req := &pl.SearchMediaItemsRequest{
		PageSize: 100,
	}
	if st != nil {
		req.Filters = &pl.Filters{
			DateFilter: &pl.DateFilter{
				Ranges: []*pl.DateRange{
					&pl.DateRange{
						StartDate: getDate(*st),
						EndDate:   getDate(time.Now()),
					},
				},
			},
		}
	}
	if pt == "" {
		glog.V(2).Info("Sending API call: initial image search by date")
	} else {
		glog.V(2).Info("Sending API call: additional image search by date")
		req.PageToken = pt
	}
	glog.V(3).Infof("Search request: %v", req)
	resp, err := s.MediaItems.Search(req).Do()
	if err != nil {
		return err
	}
	for _, i := range resp.MediaItems {
		if err := ibd.AddImage(i.Filename, i.MediaMetadata.CreationTime); err != nil {
			glog.Warningf("Got error %s adding image %v", err, i)
		}
	}

	if resp.NextPageToken == "" {
		return nil
	}
	return fillImagesByDate(s, st, ibd, resp.NextPageToken, max)
}
