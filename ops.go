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

type createRootAlbumDirectory struct {
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

func rootAlbumDir(root string) string {
	return filepath.Join(root, "albums")
}

func albumDir(root string, album string) string {
	return filepath.Join(rootAlbumDir(root), album)
}

// maybeCreateRootAlbumDir returns a pointer to an op if the directory doesn't alreayd exist
func maybeCreateRootAlbumDir(root string) *createRootAlbumDirectory {
	if _, err := os.Stat(rootAlbumDir(root)); err == nil {
		return nil
	}
	return &createRootAlbumDirectory{}
}

func (o createRootAlbumDirectory) log() string {
	return fmt.Sprintf("mkdir albums")
}

func (o createRootAlbumDirectory) run(dir string) error {
	return os.Mkdir(rootAlbumDir(dir), os.ModePerm)
}

func (o createAlbumDirectory) log() string {
	return fmt.Sprintf("mkdir %s", o.albumTitle)
}

func (o createAlbumDirectory) run(dir string) error {
	return os.Mkdir(albumDir(dir, o.albumTitle), os.ModePerm)
}

func (o removeAlbumDirectory) log() string {
	return fmt.Sprintf("rmdir %s", o.albumTitle)
}

func (o removeAlbumDirectory) run(dir string) error {
	return os.Remove(filepath.Join(albumDir(dir, o.albumTitle)))
}

func (o addAlbumLink) log() string {
	return fmt.Sprintf("ln -s %s/%s %s", o.albumTitle, o.filename, o.imagePath)
}

func (o addAlbumLink) run(dir string) error {
	d := filepath.Join(albumDir(dir, o.albumTitle), o.filename)
	return os.Symlink(o.imagePath, d)
}

func (o removeAlbumLink) log() string {
	return fmt.Sprintf("rm %s/%s", o.albumTitle, o.filename)
}

func (o removeAlbumLink) run(dir string) error {
	d := filepath.Join(albumDir(dir, o.albumTitle), o.filename)
	return os.Remove(d)
}
