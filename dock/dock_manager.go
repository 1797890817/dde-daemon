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
	"fmt"
	"sync"
	"time"

	// dbus interfaces:
	libApps "dbus/com/deepin/daemon/apps"
	"dbus/com/deepin/dde/daemon/launcher"
	libDDELauncher "dbus/com/deepin/dde/launcher"
	"dbus/com/deepin/sessionmanager"
	"dbus/com/deepin/wm"

	ddbus "pkg.deepin.io/dde/daemon/dbus"

	"gir/gio-2.0"
	"pkg.deepin.io/lib/dbus1"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil/ewmh"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/dbusutil/gsprop"
)

type Manager struct {
	PropsMu            sync.RWMutex
	Entries            AppEntries
	HideMode           gsprop.Enum `prop:"access:rw"`
	DisplayMode        gsprop.Enum `prop:"access:rw"`
	Position           gsprop.Enum `prop:"access:rw"`
	IconSize           gsprop.Uint `prop:"access:rw"`
	ShowTimeout        gsprop.Uint `prop:"access:rw"`
	HideTimeout        gsprop.Uint `prop:"access:rw"`
	DockedApps         gsprop.Strv
	HideState          HideStateType
	FrontendWindowRect *Rect

	service            *dbusutil.Service
	clientList         windowSlice
	windowInfoMap      map[xproto.Window]*WindowInfo
	windowInfoMapMutex sync.RWMutex
	settings           *gio.Settings

	activeWindow    xproto.Window
	activeWindowOld xproto.Window
	activeWindowMu  sync.Mutex

	ddeLauncherVisible   bool
	ddeLauncherVisibleMu sync.Mutex

	smartHideModeTimer *time.Timer
	smartHideModeMutex sync.Mutex

	entryCount         uint
	identifyWindowFuns []*IdentifyWindowFunc
	windowPatterns     WindowPatterns

	// dbus objects:
	launcher         *launcher.Launcher
	ddeLauncher      *libDDELauncher.Launcher
	wm               *wm.Wm
	launchedRecorder *libApps.LaunchedRecorder
	startManager     *sessionmanager.StartManager

	signals *struct {
		ServiceRestarted struct{}
		EntryAdded       struct {
			path  dbus.ObjectPath
			index int32
		}

		EntryRemoved struct {
			entryId string
		}
	}

	methods *struct {
		ActivateWindow            func() `in:"win"`
		CloseWindow               func() `in:"win"`
		MaximizeWindow            func() `in:"win"`
		MinimizeWindow            func() `in:"win"`
		MakeWindowAbove           func() `in:"win"`
		MoveWindow                func() `in:"win"`
		PreviewWindow             func() `in:"win"`
		GetEntryIDs               func() `out:"list"`
		SetFrontendWindowRect     func() `in:"x,y,width,height"`
		IsDocked                  func() `in:"desktopFile" out:"value"`
		RequestDock               func() `in:"desktopFile,index" out:"ok"`
		RequestUndock             func() `in:"desktopFile" out:"ok"`
		MoveEntry                 func() `in:"index,newIndex"`
		IsOnDock                  func() `in:"desktopFile" out:"value"`
		QueryWindowIdentifyMethod func() `in:"win" out:"identifyMethod"`
	}
}

const (
	dockSchema                     = "com.deepin.dde.dock"
	settingKeyHideMode             = "hide-mode"
	settingKeyDisplayMode          = "display-mode"
	settingKeyPosition             = "position"
	settingKeyIconSize             = "icon-size"
	settingKeyDockedApps           = "docked-apps"
	settingKeyShowTimeout          = "show-timeout"
	settingKeyHideTimeout          = "hide-timeout"
	settingKeyWinIconPreferredApps = "win-icon-preferred-apps"

	frontendWindowWmClass = "dde-dock"

	dbusServiceName = "com.deepin.dde.daemon.Dock"
	dbusPath        = "/com/deepin/dde/daemon/Dock"
	dbusInterface   = dbusServiceName
)

func newManager(service *dbusutil.Service) (*Manager, error) {
	m := new(Manager)
	m.service = service
	err := m.init()
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) destroy() {
	if m.smartHideModeTimer != nil {
		m.smartHideModeTimer.Stop()
		m.smartHideModeTimer = nil
	}

	if m.settings != nil {
		m.settings.Unref()
		m.settings = nil
	}

	if m.wm != nil {
		wm.DestroyWm(m.wm)
		m.wm = nil
	}

	if m.launcher != nil {
		launcher.DestroyLauncher(m.launcher)
		m.launcher = nil
	}

	if m.launchedRecorder != nil {
		libApps.DestroyLaunchedRecorder(m.launchedRecorder)
		m.launchedRecorder = nil
	}

	if m.startManager != nil {
		sessionmanager.DestroyStartManager(m.startManager)
		m.startManager = nil
	}

	m.service.StopExport(m)
}

func (m *Manager) launch(desktopFile string, timestamp uint32, files []string) {
	err := m.startManager.LaunchApp(desktopFile, timestamp, files)
	if err != nil {
		logger.Warningf("launch %q failed: %v", desktopFile, err)
	}
}

// ActivateWindow会激活给定id的窗口，被激活的窗口通常会成为焦点窗口。
func (m *Manager) ActivateWindow(win uint32) *dbus.Error {
	err := activateWindow(xproto.Window(win))
	if err != nil {
		logger.Warning("Activate window failed:", err)
		return dbusutil.ToError(err)
	}
	return nil
}

// CloseWindow会将传入id的窗口关闭。
func (m *Manager) CloseWindow(win uint32) *dbus.Error {
	err := ewmh.CloseWindow(XU, xproto.Window(win))
	if err != nil {
		logger.Warning("Close window failed:", err)
		return dbusutil.ToError(err)
	}
	return nil
}

func (m *Manager) MaximizeWindow(win uint32) *dbus.Error {
	err := m.ActivateWindow(win)
	if err != nil {
		return err
	}
	err1 := maximizeWindow(XU, xproto.Window(win))
	if err1 != nil {
		logger.Warning("maximize window failed:", err)
		return dbusutil.ToError(err1)
	}
	return nil
}

func (m *Manager) MinimizeWindow(win uint32) *dbus.Error {
	err := minimizeWindow(XU, xproto.Window(win))
	if err != nil {
		logger.Warning("minimize window failed:", err)
		return dbusutil.ToError(err)
	}
	return nil
}

func (m *Manager) MakeWindowAbove(win uint32) *dbus.Error {
	err := m.ActivateWindow(win)
	if err != nil {
		return err
	}

	err1 := makeWindowAbove(XU, xproto.Window(win))
	if err1 != nil {
		logger.Warning("make window above failed:", err)
		return dbusutil.ToError(err1)
	}
	return nil
}

func (m *Manager) MoveWindow(win uint32) *dbus.Error {
	err := m.ActivateWindow(win)
	if err != nil {
		return err
	}

	err1 := moveWindow(XU, xproto.Window(win))
	if err1 != nil {
		logger.Warning("move window failed:", err)
		return dbusutil.ToError(err1)
	}
	return nil
}

func (m *Manager) PreviewWindow(win uint32) *dbus.Error {
	if !ddbus.IsSessionBusActivated(m.wm.DestName) {
		logger.Warning("Deepin window manager not running, unsupported this operation")
		return nil
	}
	err := m.wm.PreviewWindow(win)
	return dbusutil.ToError(err)
}

func (m *Manager) CancelPreviewWindow() *dbus.Error {
	if !ddbus.IsSessionBusActivated(m.wm.DestName) {
		logger.Warning("Deepin window manager not running, unsupported this operation")
		return nil
	}
	err := m.wm.CancelPreviewWindow()
	return dbusutil.ToError(err)
}

// for debug
func (m *Manager) GetEntryIDs() ([]string, *dbus.Error) {
	entries := m.Entries
	entries.mu.RLock()
	list := make([]string, 0, len(entries.items))
	for _, entry := range entries.items {
		var appId string
		if entry.appInfo != nil {
			appId = entry.appInfo.GetId()
		} else {
			appId = entry.innerId
		}
		list = append(list, appId)
	}
	entries.mu.RUnlock()
	return list, nil
}

func (m *Manager) SetFrontendWindowRect(x, y int32, width, height uint32) *dbus.Error {
	if m.FrontendWindowRect.X == x &&
		m.FrontendWindowRect.Y == y &&
		m.FrontendWindowRect.Width == width &&
		m.FrontendWindowRect.Height == height {
		logger.Debug("SetFrontendWindowRect no changed")
		return nil
	}
	m.FrontendWindowRect.X = x
	m.FrontendWindowRect.Y = y
	m.FrontendWindowRect.Width = width
	m.FrontendWindowRect.Height = height
	m.service.EmitPropertyChanged(m, "FrontendWindowRect", m.FrontendWindowRect)
	m.updateHideState(false)
	return nil
}

func (m *Manager) IsDocked(desktopFilePath string) (bool, *dbus.Error) {
	entry, err := m.getDockedAppEntryByDesktopFilePath(desktopFilePath)
	if err != nil {
		return false, dbusutil.ToError(err)
	}
	return entry != nil, nil
}

func (m *Manager) RequestDock(desktopFilePath string, index int32) (bool, *dbus.Error) {
	appInfo := NewAppInfoFromFile(desktopFilePath)
	if appInfo == nil {
		return false, dbusutil.ToError(errors.New("invalid desktopFilePath"))
	}
	entry := m.Entries.GetByInnerId(appInfo.innerId)

	if entry == nil {
		entry = newAppEntry(m, appInfo.innerId, appInfo)
		err := m.exportAppEntry(entry)
		if err == nil {
			m.Entries.Insert(entry, int(index))
		}
	}
	dockResult := m.dockEntry(entry)
	return dockResult, nil
}

func (m *Manager) RequestUndock(desktopFilePath string) (bool, *dbus.Error) {
	entry, err := m.getDockedAppEntryByDesktopFilePath(desktopFilePath)
	if err != nil {
		return false, dbusutil.ToError(err)
	}
	if entry == nil {
		return false, nil
	}
	m.undockEntry(entry)
	return true, nil
}

func (m *Manager) MoveEntry(index, newIndex int32) *dbus.Error {
	err := m.Entries.Move(int(index), int(newIndex))
	if err != nil {
		logger.Warning("MoveEntry failed:", err)
		return dbusutil.ToError(err)
	}
	logger.Debug("MoveEntry ok")
	m.saveDockedApps()
	return nil
}

func (m *Manager) IsOnDock(desktopFilePath string) (bool, *dbus.Error) {
	entry, err := m.Entries.GetByDesktopFilePath(desktopFilePath)
	if err != nil {
		return false, dbusutil.ToError(err)
	}
	return entry != nil, nil
}

func (m *Manager) QueryWindowIdentifyMethod(wid uint32) (string, *dbus.Error) {
	m.Entries.mu.RLock()
	defer m.Entries.mu.RUnlock()

	for _, entry := range m.Entries.items {
		winInfo, ok := entry.windows[xproto.Window(wid)]
		if ok {
			if winInfo.appInfo != nil {
				return winInfo.appInfo.identifyMethod, nil
			} else {
				return "Failed", nil
			}
		}
	}
	return "", dbusutil.ToError(fmt.Errorf("window %d not found", wid))
}
