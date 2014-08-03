/**
 * Copyright (c) 2013 ~ 2014 Deepin, Inc.
 *               2013 ~ 2014 Xu FaSheng
 *
 * Author:      Xu FaSheng <fasheng.xu@gmail.com>
 * Maintainer:  Xu FaSheng <fasheng.xu@gmail.com>
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, see <http://www.gnu.org/licenses/>.
 **/

package grub2

import (
	"fmt"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xwindow"
	"strconv"
	"strings"
)

func quoteString(str string) string {
	return strconv.Quote(str)
}

func unquoteString(str string) string {
	if strings.HasPrefix(str, `"`) && strings.HasSuffix(str, `"`) {
		s, _ := strconv.Unquote(str)
		return s
	} else if strings.HasPrefix(str, `'`) && strings.HasSuffix(str, `'`) {
		return str[1 : len(str)-1]
	}
	return str
}

func isStringInArray(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func appendStrArrayUnique(a1 []string, a2 ...string) (a []string) {
	a = a1
	for _, s := range a2 {
		if !isStringInArray(s, a) {
			a = append(a, s)
		}
	}
	return
}

// Get all screen's best resolution and choose a smaller one for there
// is no screen is primary.
func getPrimaryScreenBestResolutionStr() (r string) {
	w, h := getPrimaryScreenBestResolution()
	r = fmt.Sprintf("%dx%d", w, h)
	return
}
func getPrimaryScreenBestResolution() (w uint16, h uint16) {
	// if connect to x failed, just return 1024x768
	w, h = 1024, 768

	XU, err := xgbutil.NewConn()
	if err != nil {
		return
	}
	err = randr.Init(XU.Conn())
	if err != nil {
		return
	}
	_, err = randr.QueryVersion(XU.Conn(), 1, 4).Reply()
	if err != nil {
		return
	}
	Root := xproto.Setup(XU.Conn()).DefaultScreen(XU.Conn()).Root
	resources, err := randr.GetScreenResources(XU.Conn(), Root).Reply()
	if err != nil {
		return
	}

	bestModes := make([]uint32, 0)
	for _, output := range resources.Outputs {
		reply, err := randr.GetOutputInfo(XU.Conn(), output, 0).Reply()
		if err == nil && reply.NumModes > 1 {
			bestModes = append(bestModes, uint32(reply.Modes[0]))
		}
	}

	w, h = 0, 0
	for _, m := range resources.Modes {
		for _, id := range bestModes {
			if id == m.Id {
				bw, bh := m.Width, m.Height
				if w == 0 || h == 0 {
					w, h = bw, bh
				} else if uint32(bw)*uint32(bh) < uint32(w)*uint32(h) {
					w, h = bw, bh
				}
			}
		}
	}

	if w == 0 || h == 0 {
		// get resource failed, use root window's geometry
		rootRect := xwindow.RootGeometry(XU)
		w, h = uint16(rootRect.Width()), uint16(rootRect.Height())
	}

	if w == 0 || h == 0 {
		w, h = 1024, 768 // default value
	}

	logger.Debugf("primary screen's best resolution is %dx%d", w, h)
	return
}

func delta(v1, v2 float64) float64 {
	if v1 > v2 {
		return v1 - v2
	}
	return v2 - v1
}

// "0" -> "0", "1->2" -> "1", "Parent Tiltle>Child Title" -> "Parent Title"
func convertToSimpleEntry(entry string) (simpleEntry string) {
	i := strings.Index(entry, ">")
	if i >= 0 {
		simpleEntry = entry[0:i]
	} else {
		simpleEntry = entry
	}
	return
}

func parseGfxmode(gfxmode string) (w, h uint16) {
	w, h, err := doParseGfxmode(gfxmode)
	if err != nil {
		logger.Error(err)
		w, h = getPrimaryScreenBestResolution() // default value
	}
	return
}

func doParseGfxmode(gfxmode string) (w, h uint16, err error) {
	// check if contains ',' or ';', if so, just split first field as gfxmode
	if strings.Contains(gfxmode, ",") {
		gfxmode = strings.Split(gfxmode, ",")[0]
	} else if strings.Contains(gfxmode, ";") {
		gfxmode = strings.Split(gfxmode, ";")[0]
	}

	if gfxmode == "auto" {
		// just return screen resolution if gfxmode is "auto"
		w, h = getPrimaryScreenBestResolution()
		return
	}

	a := strings.Split(gfxmode, "x")
	if len(a) < 2 {
		err = fmt.Errorf("gfxmode format error, %s", gfxmode)
		return
	}

	// parse width
	tmpw, err := strconv.ParseUint(a[0], 10, 16)
	if err != nil {
		return
	}

	// parse height
	tmph, err := strconv.ParseUint(a[1], 10, 16)
	if err != nil {
		return
	}

	w = uint16(tmpw)
	h = uint16(tmph)
	return
}
