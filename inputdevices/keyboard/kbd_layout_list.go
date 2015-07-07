/**
 * Copyright (c) 2011 ~ 2013 Deepin, Inc.
 *               2011 ~ 2013 jouyouyun
 *
 * Author:      jouyouyun <jouyouwen717@gmail.com>
 * Maintainer:  jouyouyun <jouyouwen717@gmail.com>
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

package keyboard

import (
	"encoding/xml"
	"io/ioutil"
	. "pkg.deepin.io/lib/gettext"
)

const (
	kbdKeyLayoutDelim = ";"
)

type XKBConfigRegister struct {
	LayoutList XLayoutList `xml:"layoutList"`
}

type XLayoutList struct {
	Layout []XLayout `xml:"layout"`
}

type XLayout struct {
	ConfigItem  XConfigItem  `xml:"configItem"`
	VariantList XVariantList `xml:"variantList"`
}

type XConfigItem struct {
	Name        string `xml:"name"`
	Description string `xml:"description"`
}

type XVariantList struct {
	Variant []XVariant `xml:"variant"`
}

type XVariant struct {
	ConfigItem XConfigItem `xml:"configItem"`
}

const (
	kbdKeyLayoutXml = "/usr/share/X11/xkb/rules/base.xml"
)

func parseXML(filename string) (XKBConfigRegister, error) {
	var v XKBConfigRegister
	xmlByte, err := ioutil.ReadFile(filename)
	if err != nil {
		return v, err
	}

	err = xml.Unmarshal(xmlByte, &v)
	if err != nil {
		return v, err
	}

	return v, nil
}

func getLayoutListByFile(filename string) (map[string]string, error) {
	xmlData, err := parseXML(filename)
	if err != nil {
		return nil, err
	}

	layouts := make(map[string]string)
	for _, layout := range xmlData.LayoutList.Layout {
		firstName := layout.ConfigItem.Name
		desc := layout.ConfigItem.Description
		layouts[firstName+kbdKeyLayoutDelim] = DGettext("xkeyboard-config", desc)

		variants := layout.VariantList.Variant
		for _, v := range variants {
			lastName := v.ConfigItem.Name
			descTmp := v.ConfigItem.Description
			layouts[firstName+kbdKeyLayoutDelim+lastName] = Tr(descTmp)
		}
	}

	return layouts, nil
}
