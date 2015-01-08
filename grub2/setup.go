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

// Setup grub2 environment, regenerate configure and theme if need, don't depends on dbus
func (grub *Grub2) Setup(gfxmode string) {
	runWithoutDbus = true

	// do not call grub.readEntries() here, for that
	// "/boot/grub/grub.cfg" may not exists
	grub.readSettings()
	grub.fixSettings()
	grub.fixSettingDistro()

	// setup gfxmode
	if len(gfxmode) == 0 {
		grub.setSettingGfxmode(grub.config.Resolution)
	} else {
		grub.setSettingGfxmode(gfxmode)
	}

	// write settings
	grub.writeSettings()
	// reset NeedUpdate flag for that will run update-grub always
	// after setup grub
	grub.config.NeedUpdate = false
	grub.config.save()

	// setup theme and generate theme background
	grub.SetupTheme(gfxmode)
}

func (grub *Grub2) SetupTheme(gfxmode string) {
	runWithoutDbus = true
	grub.config.loadOrSaveConfig()
	if len(gfxmode) == 0 {
		gfxmode = grub.config.Resolution
	}
	w, h := parseGfxmode(gfxmode)
	doGenerateThemeBackground(w, h)
}

func doWriteGrubSettings(fileContent string) {
	ge := NewGrub2Ext()
	ge.DoWriteGrubSettings(fileContent)
}

func doGenerateThemeBackground(w, h uint16) {
	ge := NewGrub2Ext()
	ge.DoGenerateThemeBackground(w, h)
}
