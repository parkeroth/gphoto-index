package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type operation interface {
	run(dir string) error
	log() string
}

type createAlbumDirectory struct {
	albumTitle string
}

type removeAlbumDirectory struct {
	albumTitle string
}

type addAlbumLink struct {
	albumTitle, imagePath, filename string
}

type removeAlbumLink struct {
	albumTitle, filename string
}

func (o createAlbumDirectory) log() string {
	return fmt.Sprintf("mkdir %s", o.albumTitle)
}

func (o createAlbumDirectory) run(dir string) error {
	d := filepath.Join(dir, o.albumTitle)
	return os.Mkdir(d, os.ModePerm)
}

func (o removeAlbumDirectory) log() string {
	return fmt.Sprintf("rmdir %s", o.albumTitle)
}

func (o removeAlbumDirectory) run(dir string) error {
	d := filepath.Join(dir, o.albumTitle)
	return os.Remove(d)
}

func (o addAlbumLink) log() string {
	return fmt.Sprintf("ln -s %s/%s %s", o.albumTitle, o.filename, o.imagePath)
}

func (o addAlbumLink) run(dir string) error {
	d := filepath.Join(dir, o.albumTitle, o.filename)
	return os.Symlink(o.imagePath, d)
}

func (o removeAlbumLink) log() string {
	return fmt.Sprintf("rm %s/%s", o.albumTitle, o.filename)
}

func (o removeAlbumLink) run(dir string) error {
	d := filepath.Join(dir, o.albumTitle, o.filename)
	return os.Remove(d)
}
