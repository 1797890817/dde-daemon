/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
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

package keybinding

import (
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.helper.backlight"
	"github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.inputdevices"
	"github.com/linuxdeepin/go-dbus-factory/com.deepin.sessionmanager"

	"gir/gio-2.0"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/util/keysyms"
	"pkg.deepin.io/dde/daemon/keybinding/shortcuts"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/dbusutil/gsprop"
	"pkg.deepin.io/lib/dbusutil/proxy"
	"pkg.deepin.io/lib/gsettings"
	"pkg.deepin.io/lib/xdg/basedir"
)

const (
	// shortcut signals:
	shortcutSignalChanged = "Changed"
	shortcutSignalAdded   = "Added"
	shortcutSignalDeleted = "Deleted"

	gsSchemaKeyboard          = "com.deepin.dde.keyboard"
	gsKeyNumLockState         = "numlock-state"
	gsKeySaveNumLockState     = "save-numlock-state"
	gsKeyShortcutSwitchLayout = "shortcut-switch-layout"
	gsKeyShowCapsLockOSD      = "capslock-toggle"

	gsSchemaSystem   = "com.deepin.dde.keybinding.system"
	gsSchemaMediaKey = "com.deepin.dde.keybinding.mediakey"
	gsSchemaGnomeWM  = "com.deepin.wrap.gnome.desktop.wm.keybindings"

	customConfigFile = "deepin/dde-daemon/keybinding/custom.ini"
)

type Manager struct {
	service *dbusutil.Service
	// properties
	NumLockState         gsprop.Enum
	ShortcutSwitchLayout gsprop.Uint `prop:"access:rw"`

	conn       *x.Conn
	keySymbols *keysyms.KeySymbols

	gsKeyboard *gio.Settings
	gsSystem   *gio.Settings
	gsMediaKey *gio.Settings
	gsGnomeWM  *gio.Settings

	enableListenGSettings bool

	customShortcutManager *shortcuts.CustomShortcutManager

	sessionSigLoop  *dbusutil.SignalLoop
	systemSigLoop   *dbusutil.SignalLoop
	startManager    *sessionmanager.StartManager
	backlightHelper *backlight.Backlight
	keyboard        *inputdevices.Keyboard
	keyboardLayout  string

	// controllers
	audioController       *AudioController
	mediaPlayerController *MediaPlayerController
	displayController     *DisplayController
	kbdLightController    *KbdLightController
	touchPadController    *TouchPadController

	shortcutManager *shortcuts.ShortcutManager
	// shortcut action handlers
	handlers            []shortcuts.KeyEventFunc
	lastKeyEventTime    time.Time
	grabScreenKeystroke *shortcuts.Keystroke

	// for switch kbd layout
	switchKbdLayoutState SKLState
	sklWaitQuit          chan int

	signals *struct {
		Added, Deleted, Changed struct {
			id  string
			typ int32
		}

		KeyEvent struct {
			pressed   bool
			keystroke string
		}
	}

	methods *struct {
		AddCustomShortcut         func() `in:"name,action,keystroke" out:"id,type"`
		AddShortcutKeystroke      func() `in:"id,type,keystroke"`
		ClearShortcutKeystrokes   func() `in:"id,type"`
		DeleteCustomShortcut      func() `in:"id"`
		DeleteShortcutKeystroke   func() `in:"id,type,keystroke"`
		GetShortcut               func() `in:"id,type" out:"shortcut"`
		ListAllShortcuts          func() `out:"shortcuts"`
		ListShortcutsByType       func() `in:"type" out:"shortcuts"`
		LookupConflictingShortcut func() `in:"keystroke" out:"shortcut"`
		ModifyCustomShortcut      func() `in:"id,name,cmd,keystroke"`
		SetNumLockState           func() `in:"state"`

		// deprecated
		Add            func() `in:"name,action,keystroke" out:"ret0,ret1"`
		Query          func() `in:"id,type" out:"shortcut"`
		List           func() `out:"shortcuts"`
		Delete         func() `in:"id,type"`
		Disable        func() `in:"id,type"`
		CheckAvaliable func() `in:"keystroke" out:"available,shortcut"`
		ModifiedAccel  func() `in:"id,type,keystroke,add" out:"ret0,ret1"`
	}
}

// SKLState Switch keyboard Layout state
type SKLState uint

const (
	SKLStateNone SKLState = iota
	SKLStateWait
	SKLStateOSDShown
)

func newManager(service *dbusutil.Service) (*Manager, error) {
	conn, err := x.NewConn()
	if err != nil {
		return nil, err
	}

	systemConn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	var m = Manager{
		service:               service,
		enableListenGSettings: true,
		conn:       conn,
		keySymbols: keysyms.NewKeySymbols(conn),
	}

	m.sessionSigLoop = dbusutil.NewSignalLoop(service.Conn(), 10)
	m.systemSigLoop = dbusutil.NewSignalLoop(systemConn, 10)

	m.gsKeyboard = gio.NewSettings(gsSchemaKeyboard)
	m.NumLockState.Bind(m.gsKeyboard, gsKeyNumLockState)
	m.ShortcutSwitchLayout.Bind(m.gsKeyboard, gsKeyShortcutSwitchLayout)
	return &m, nil
}

func (m *Manager) systemConn() *dbus.Conn {
	return m.systemSigLoop.Conn()
}

func (m *Manager) sessionConn() *dbus.Conn {
	return m.sessionSigLoop.Conn()
}

func (m *Manager) init() {
	m.sessionSigLoop.Start()
	m.systemSigLoop.Start()

	if m.gsKeyboard.GetBoolean(gsKeySaveNumLockState) {
		nlState := NumLockState(m.NumLockState.Get())
		if nlState == NumLockUnknown {
			state, err := queryNumLockState(m.conn)
			if err != nil {
				logger.Warning("queryNumLockState failed:", err)
			} else {
				m.NumLockState.Set(int32(state))
			}
		} else {
			err := setNumLockState(m.conn, m.keySymbols, nlState)
			if err != nil {
				logger.Warning("setNumLockState failed:", err)
			}
		}
	}

	// init settings
	m.gsSystem = gio.NewSettings(gsSchemaSystem)
	m.gsMediaKey = gio.NewSettings(gsSchemaMediaKey)
	m.gsGnomeWM = gio.NewSettings(gsSchemaGnomeWM)

	m.shortcutManager = shortcuts.NewShortcutManager(m.conn, m.keySymbols, m.handleKeyEvent)
	m.shortcutManager.AddSpecial()
	m.shortcutManager.AddSystem(m.gsSystem)
	m.shortcutManager.AddMedia(m.gsMediaKey)
	m.shortcutManager.AddWM(m.gsGnomeWM)

	customConfigFilePath := filepath.Join(basedir.GetUserConfigDir(), customConfigFile)
	m.customShortcutManager = shortcuts.NewCustomShortcutManager(customConfigFilePath)
	m.shortcutManager.AddCustom(m.customShortcutManager)

	sessionConn := m.sessionConn()
	m.audioController = NewAudioController(sessionConn)
	m.mediaPlayerController = NewMediaPlayerController(m.systemSigLoop, sessionConn)

	m.backlightHelper = backlight.NewBacklight(m.systemConn())
	m.startManager = sessionmanager.NewStartManager(sessionConn)
	m.keyboard = inputdevices.NewKeyboard(sessionConn)
	m.keyboard.InitSignalExt(m.sessionSigLoop, true)
	m.keyboard.CurrentLayout().ConnectChanged(func(hasValue bool, layout string) {
		if !hasValue {
			return
		}
		if m.keyboardLayout != layout {
			m.keyboardLayout = layout
			logger.Debug("keyboard layout changed:", layout)
			m.shortcutManager.NotifyLayoutChanged()
		}
	})

	m.displayController = NewDisplayController(m.backlightHelper, sessionConn)
	m.kbdLightController = NewKbdLightController(m.backlightHelper)
	m.touchPadController = NewTouchPadController(sessionConn)
}

func (m *Manager) destroy() {
	m.service.StopExport(m)

	if m.shortcutManager != nil {
		m.shortcutManager.Destroy()
		m.shortcutManager = nil
	}

	// destroy settings
	if m.gsSystem != nil {
		m.gsSystem.Unref()
		m.gsSystem = nil
	}

	if m.gsMediaKey != nil {
		m.gsMediaKey.Unref()
		m.gsMediaKey = nil
	}

	if m.gsGnomeWM != nil {
		m.gsGnomeWM.Unref()
		m.gsGnomeWM = nil
	}

	if m.mediaPlayerController != nil {
		m.mediaPlayerController.Destroy()
		m.mediaPlayerController = nil
	}

	if m.keyboard != nil {
		m.keyboard.RemoveHandler(proxy.RemoveAllHandlers)
		m.keyboard = nil
	}

	if m.sessionSigLoop != nil {
		m.sessionSigLoop.Stop()
		m.sessionSigLoop = nil
	}

	if m.systemSigLoop != nil {
		m.systemSigLoop.Stop()
		m.systemSigLoop = nil
	}
}

func (m *Manager) handleKeyEvent(ev *shortcuts.KeyEvent) {
	now := time.Now()
	duration := now.Sub(m.lastKeyEventTime)
	logger.Debug("duration:", duration)
	if 0 < duration && duration < 200*time.Millisecond {
		logger.Debug("handleKeyEvent ignore key event")
		return
	}
	m.lastKeyEventTime = now

	logger.Debugf("handleKeyEvent ev: %#v", ev)
	action := ev.Shortcut.GetAction()
	logger.Debugf("shortcut id: %s, type: %v, action: %#v",
		ev.Shortcut.GetId(), ev.Shortcut.GetType(), action)
	if action == nil {
		logger.Warning("action is nil")
		return
	}
	if handler := m.handlers[int(action.Type)]; handler != nil {
		handler(ev)
	} else {
		logger.Warning("handler is nil")
	}
}

func shouldEmitSignalChanged(shortcut shortcuts.Shortcut) bool {
	return shortcut.GetType() == shortcuts.ShortcutTypeCustom
}

func (m *Manager) emitShortcutSignal(signalName string, shortcut shortcuts.Shortcut) {
	logger.Debug("emit DBus signal", signalName, shortcut.GetId(), shortcut.GetType())
	m.service.Emit(m, signalName, shortcut.GetId(), shortcut.GetType())
}

func (m *Manager) enableListenGSettingsChanged(val bool) {
	m.enableListenGSettings = val
}

func (m *Manager) listenGSettingsChanged(schema string, settings *gio.Settings, type0 int32) {
	gsettings.ConnectChanged(schema, "*", func(key string) {
		if !m.enableListenGSettings {
			return
		}

		shortcut := m.shortcutManager.GetByIdType(key, type0)
		if shortcut == nil {
			return
		}

		keystrokes := settings.GetStrv(key)
		m.shortcutManager.ModifyShortcutKeystrokes(shortcut, shortcuts.ParseKeystrokes(keystrokes))
		m.emitShortcutSignal(shortcutSignalChanged, shortcut)
	})
}

func (m *Manager) execCmd(cmd string) error {
	if len(cmd) == 0 {
		logger.Debug("cmd is empty")
		return nil
	}
	if strings.HasPrefix(cmd, "dbus-send ") {
		logger.Debug("run cmd:", cmd)
		return exec.Command("sh", "-c", cmd).Run()
	}

	logger.Debug("startdde run cmd:", cmd)
	return m.startManager.RunCommand(0, "sh", []string{"-c", cmd})
}

func (m *Manager) eliminateKeystrokeConflict() {
	for _, ks := range m.shortcutManager.ConflictingKeystrokes {
		shortcut := ks.Shortcut
		logger.Infof("eliminate conflict shortcut: %s keystroke: %s",
			ks.Shortcut.GetUid(), ks)
		m.DeleteShortcutKeystroke(shortcut.GetId(), shortcut.GetType(), ks.String())
	}

	m.shortcutManager.ConflictingKeystrokes = nil
	m.shortcutManager.EliminateConflictDone = true
}
