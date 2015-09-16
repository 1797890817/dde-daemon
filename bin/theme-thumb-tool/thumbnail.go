package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"path"
	"pkg.deepin.io/dde/api/themes"
	"pkg.deepin.io/dde/api/thumbnails/cursor"
	"pkg.deepin.io/dde/api/thumbnails/gtk"
	"pkg.deepin.io/dde/api/thumbnails/icon"
	"pkg.deepin.io/dde/api/thumbnails/images"
	"pkg.deepin.io/dde/daemon/appearance/background"
	"pkg.deepin.io/lib/graphic"
	"time"
)

const (
	thumbBgDir = "/var/cache/appearance/thumbnail/background"

	defaultWidth  int = 128
	defaultHeight     = 72
)

func genAllThumbnails(force bool) {
	genGtkThumbnails(force)
	genIconThumbnails(force)
	genCursorThumbnails(force)
	genBgThumbnails(force)
}

func genGtkThumbnails(force bool) {
	list := themes.ListGtkTheme()
	for _, v := range list {
		_, err := gtk.GenThumbnail(path.Join(v, "index.theme"),
			getThumbBg(), defaultWidth, defaultHeight, force)
		if err != nil {
			fmt.Printf("Gen '%s' thumbnail failed: %v\n", v, err)
			continue
		}
	}
}

func genIconThumbnails(force bool) {
	list := themes.ListIconTheme()
	for _, v := range list {
		_, err := icon.GenThumbnail(path.Join(v, "index.theme"),
			getThumbBg(), defaultWidth, defaultHeight, force)
		if err != nil {
			fmt.Printf("Gen '%s' thumbnail failed: %v\n", v, err)
			continue
		}
	}
}

func genCursorThumbnails(force bool) {
	list := themes.ListCursorTheme()
	for _, v := range list {
		_, err := cursor.GenThumbnail(path.Join(v, "cursor.theme"),
			getThumbBg(), defaultWidth, defaultHeight, force)
		if err != nil {
			fmt.Printf("Gen '%s' thumbnail failed: %v\n", v, err)
			continue
		}
	}
}

func genBgThumbnails(force bool) {
	infos := background.ListBackground()
	for _, info := range infos {
		_, err := images.GenThumbnail(info.URI,
			defaultWidth, defaultHeight, force)
		if err != nil {
			fmt.Printf("Gen '%s' thumbnail failed: %v\n", info.URI, err)
			continue
		}
	}
}

func getThumbBg() string {
	var imgs = getImagesInDir()
	if len(imgs) == 0 {
		return ""
	}

	rand.Seed(time.Now().UnixNano())
	idx := rand.Intn(len(imgs))
	return imgs[idx]
}

func getImagesInDir() []string {
	finfos, err := ioutil.ReadDir(thumbBgDir)
	if err != nil {
		return nil
	}

	var imgs []string
	for _, finfo := range finfos {
		tmp := path.Join(thumbBgDir, finfo.Name())
		if !graphic.IsSupportedImage(tmp) {
			continue
		}
		imgs = append(imgs, tmp)
	}
	return imgs
}