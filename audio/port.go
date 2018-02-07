/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package audio

import (
	"fmt"
	"pkg.deepin.io/lib/pulse"
)

type Port struct {
	Name        string
	Description string
	Available   byte // Unknow:0, No:1, Yes:2
}

func (p Port) String() string {
	availableStr := "Invalid"
	switch int(p.Available) {
	case pulse.AvailableTypeUnknow:
		availableStr = "Unknow"
	case pulse.AvailableTypeNo:
		availableStr = "No"
	case pulse.AvailableTypeYes:
		availableStr = "Yes"
	}
	return fmt.Sprintf("<Port name=%q desc=%q available=%s>", p.Name, p.Description, availableStr)
}

func toPort(v pulse.PortInfo) Port {
	return Port{
		Name:        v.Name,
		Description: v.Description,
		Available:   byte(v.Available),
	}
}

// return port and whether found
func getPortByName(ports []Port, name string) (Port, bool) {
	if name == "" {
		return Port{}, false
	}
	for _, port := range ports {
		if port.Name == name {
			return port, true
		}
	}
	return Port{}, false
}

func portsEqual(a, b []Port) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
