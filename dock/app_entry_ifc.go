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

package dock

import (
	"errors"

	"github.com/linuxdeepin/go-x11-client/util/wm/icccm"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
)

func (e *AppEntry) GetInterfaceName() string {
	return entryDBusInterface
}

func (entry *AppEntry) Activate(timestamp uint32) *dbus.Error {
	logger.Debug("Activate timestamp:", timestamp)
	m := entry.manager
	if HideModeType(m.HideMode.Get()) == HideModeSmartHide {
		m.setPropHideState(HideStateShow)
		m.updateHideState(true)
	}

	entry.PropsMu.RLock()
	hasWindow := entry.hasWindow()
	entry.PropsMu.RUnlock()

	if !hasWindow {
		entry.launchApp(timestamp)
		return nil
	}

	if entry.current == nil {
		err := errors.New("entry.current is nil")
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	win := entry.current.window
	state, err := globalEwmhConn.GetWMState(win).Reply(globalEwmhConn)
	if err != nil {
		logger.Warning("Get ewmh wmState failed win:", win)
		return dbusutil.ToError(err)
	}

	if atomsContains(state, atomNetWmStateFocused) {
		s, err := globalIcccmConn.GetWMState(win).Reply(globalIcccmConn)
		if err != nil {
			logger.Warning("Get icccm WmState failed win:", win)
			return dbusutil.ToError(err)
		}
		switch s.State {
		case icccm.StateIconic:
			activateWindow(win)
		case icccm.StateNormal:
			if len(entry.windows) == 1 {
				minimizeWindow(win)
			} else if entry.manager.getActiveWindow() == win {
				nextWin := entry.findNextLeader()
				activateWindow(nextWin)
			}
		}
	} else {
		activateWindow(win)
	}
	return nil
}

func (e *AppEntry) HandleMenuItem(timestamp uint32, id string) *dbus.Error {
	logger.Debugf("HandleMenuItem id: %q timestamp: %v", id, timestamp)

	e.PropsMu.RLock()
	menu := e.coreMenu
	e.PropsMu.RUnlock()

	if menu != nil {
		err := menu.HandleAction(id, timestamp)
		return dbusutil.ToError(err)
	}
	logger.Warning("HandleMenuItem failed: entry.coreMenu is nil")
	return nil
}

func (e *AppEntry) HandleDragDrop(timestamp uint32, files []string) *dbus.Error {
	logger.Debugf("handle drag drop files: %v, timestamp: %v", files, timestamp)

	ai := e.appInfo
	if ai != nil {
		e.manager.launch(ai.GetFileName(), timestamp, files)
	} else {
		logger.Warning("not supported")
	}
	return nil
}

// RequestDock 驻留
func (entry *AppEntry) RequestDock() *dbus.Error {
	entry.manager.dockEntry(entry)
	return nil
}

// RequestUndock 取消驻留
func (entry *AppEntry) RequestUndock() *dbus.Error {
	entry.manager.undockEntry(entry)
	return nil
}

func (entry *AppEntry) PresentWindows() *dbus.Error {
	entry.PropsMu.RLock()
	windowIds := entry.getWindowIds()
	entry.PropsMu.RUnlock()
	if len(windowIds) > 0 {
		entry.manager.wm.PresentWindows(windowIds)
	}
	return nil
}

func (entry *AppEntry) NewInstance(timestamp uint32) *dbus.Error {
	entry.launchApp(timestamp)
	return nil
}

func (entry *AppEntry) Check() *dbus.Error {
	entry.PropsMu.RLock()
	winInfoSlice := entry.getWindowInfoSlice()
	entry.PropsMu.RUnlock()

	for _, winInfo := range winInfoSlice {
		entry.manager.attachOrDetachWindow(winInfo)
	}
	return nil
}

func (entry *AppEntry) ForceQuit() *dbus.Error {
	entry.PropsMu.RLock()
	winInfoSlice := entry.getWindowInfoSlice()
	entry.PropsMu.RUnlock()

	for _, winInfo := range winInfoSlice {
		killClient(winInfo.window)
	}
	return nil
}
