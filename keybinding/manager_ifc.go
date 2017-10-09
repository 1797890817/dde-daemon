/*
 * Copyright (C) 2016 ~ 2017 Deepin Technology Co., Ltd.
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
	"errors"
	"fmt"
	"strings"

	"pkg.deepin.io/dde/daemon/keybinding/shortcuts"
	"pkg.deepin.io/lib/dbus"
)

const (
	dbusDest     = "com.deepin.daemon.Keybinding"
	bindDBusPath = "/com/deepin/daemon/Keybinding"
	bindDBusIFC  = "com.deepin.daemon.Keybinding"
)

func (*Manager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       dbusDest,
		ObjectPath: bindDBusPath,
		Interface:  bindDBusIFC,
	}
}

// Reset reset all shortcut
func (m *Manager) Reset() {
	m.shortcutManager.UngrabAll()

	m.enableListenGSettingsChanged(false)
	// reset all gsettings
	resetGSettings(m.gsSystem)
	resetGSettings(m.gsMediaKey)
	resetGSettings(m.gsGnomeWM)

	// disable all custom shortcuts
	m.customShortcutManager.DisableAll()

	changes := m.shortcutManager.ReloadAllShortcutAccels()
	m.enableListenGSettingsChanged(true)
	m.shortcutManager.GrabAll()
	for _, shortcut := range changes {
		m.emitShortcutSignal(shortcutSignalChanged, shortcut)
	}
}

// List list all shortcut
func (m *Manager) List() string {
	list := m.shortcutManager.List()
	ret, err := doMarshal(list)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return ret
}

// Add add custom shortcut
//
// name: accel name
// action: accel command line
// accel: the binded accel, ignored
// ret0: the shortcut id
// ret1: whether accel conflict, if true, ret0 is conflict id
// ret2: error info
func (m *Manager) Add(name, action, accel string) (string, bool, error) {
	logger.Debugf("Add custom key: %q %q %q", name, action, accel)
	ap, err := shortcuts.ParseStandardAccel(accel)
	if err != nil {
		return "", false, err
	}
	shortcut, err := m.customShortcutManager.Add(name, action, []shortcuts.ParsedAccel{ap})
	if err != nil {
		return "", false, err
	}
	m.shortcutManager.Add(shortcut)
	m.emitShortcutSignal(shortcutSignalAdded, shortcut)
	return "", false, nil
}

// Delete delete shortcut by id and type
//
// id: the specail id
// ty: the special type
// ret0: error info
func (m *Manager) Delete(id string, ty int32) error {
	if ty != shortcuts.ShortcutTypeCustom {
		return ErrInvalidShortcutType{ty}
	}

	shortcut := m.shortcutManager.GetByIdType(id, ty)
	if err := m.customShortcutManager.Delete(shortcut.GetId()); err != nil {
		return err
	}
	m.shortcutManager.Delete(shortcut)
	m.emitShortcutSignal(shortcutSignalDeleted, shortcut)
	return nil
}

// Disable cancel the special id accels
func (m *Manager) Disable(id string, ty int32) error {
	shortcut := m.shortcutManager.GetByIdType(id, ty)
	if shortcut == nil {
		return ErrShortcutNotFound{id, ty}
	}
	m.shortcutManager.ModifyShortcutAccels(shortcut, nil)
	return shortcut.SaveAccels()
}

type ErrInvalidShortcutType struct {
	Type int32
}

func (err ErrInvalidShortcutType) Error() string {
	return fmt.Sprintf("shortcut type %v is invalid", err.Type)
}

type ErrShortcutNotFound struct {
	Id   string
	Type int32
}

func (err ErrShortcutNotFound) Error() string {
	return fmt.Sprintf("shortcut id %q type %v is not found", err.Id, err.Type)
}

var errTypeAssertionFail = errors.New("type assertion failed")

// CheckAvaliable check the accel whether conflict
// CheckAvaliable 检查快捷键序列是否可用
// 返回值1 是否可用;
// 返回值2 与之冲突的快捷键的详细信息，是JSON字符串。如果没有冲突，则为空字符串。
func (m *Manager) CheckAvaliable(accelStr string) (bool, string, error) {
	pa, err := shortcuts.ParseStandardAccel(accelStr)
	if err != nil {
		// parse accel error
		return false, "", err
	}

	accel, err := m.shortcutManager.FindConflictingAccel(pa)
	if err != nil {
		// pa.ParseKey error
		return false, "", err
	}
	if accel != nil {
		detail, err := doMarshal(accel.Shortcut)
		if err != nil {
			return false, "", err
		}
		return false, detail, nil
	}
	return true, "", nil
}

// ModifyCustomShortcut modify custom shortcut
//
// id: shortcut id
// name: new name
// cmd: new commandline
// accelStr: new accel
func (m *Manager) ModifyCustomShortcut(id, name, cmd, accelStr string) error {
	logger.Debugf("ModifyCustomShorcut id: %q, name: %q, cmd: %q, accel: %q", id, name, cmd, accelStr)
	const ty = shortcuts.ShortcutTypeCustom
	// get the shortcut
	shortcut := m.shortcutManager.GetByIdType(id, ty)
	if shortcut == nil {
		return ErrShortcutNotFound{id, ty}
	}
	cshorcut, ok := shortcut.(*shortcuts.CustomShortcut)
	if !ok {
		return errTypeAssertionFail
	}

	var accels []shortcuts.ParsedAccel
	if accelStr != "" {
		// check conflicting
		pa, err := shortcuts.ParseStandardAccel(accelStr)
		confAccel, err := m.shortcutManager.FindConflictingAccel(pa)
		if err != nil {
			return err
		}
		if confAccel != nil {
			confShortcut := confAccel.Shortcut
			if confShortcut.GetId() != id || confShortcut.GetType() != ty {
				return fmt.Errorf("found conflict with other shortcut id: %q, type: %v",
					confShortcut.GetId(), confShortcut.GetType())
			}
			// else shorcut and confShortcut are the same shortcut
		}
		accels = []shortcuts.ParsedAccel{pa}
	}

	// modify then save
	cshorcut.Name = name
	cshorcut.Cmd = cmd
	m.shortcutManager.ModifyShortcutAccels(shortcut, accels)
	m.emitShortcutSignal(shortcutSignalChanged, shortcut)
	return cshorcut.Save()
}

var errShortcutAccelsUnmodifiable = errors.New("accels of this shortcut is unmodifiable")

// ModifiedAccel modify shortcut accel
//
// id: the special id
// ty: the special type
// accelStr: new accel
// grabed: if true, add accel for the special id; else delete it
// ret0: always equal false
// ret1: always equal ""
// ret2: error
func (m *Manager) ModifiedAccel(id string, ty int32, accelStr string, grabed bool) (bool, string, error) {
	logger.Debug("Manager.ModifiedAccel", id, ty, accelStr, grabed)
	shortcut := m.shortcutManager.GetByIdType(id, ty)
	if shortcut == nil {
		return false, "", ErrShortcutNotFound{id, ty}
	}
	if !shortcut.GetAccelsModifiable() {
		return false, "", errShortcutAccelsUnmodifiable
	}

	pa, err := shortcuts.ParseStandardAccel(accelStr)
	if err != nil {
		// parse accel error
		return false, "", err
	}

	logger.Debugf("pa: %#v", pa)

	if !grabed {
		m.shortcutManager.RemoveShortcutAccel(shortcut, pa)
		m.emitShortcutSignal(shortcutSignalChanged, shortcut)
		shortcut.SaveAccels()
		return false, "", nil
	}

	// check pa.Key valid
	_, err = pa.QueryKey(m.keySymbols)
	if err != nil {
		return false, "", err
	}

	if ty == shortcuts.ShortcutTypeWM && pa.Mods == 0 {
		keyLower := strings.ToLower(pa.Key)
		if keyLower == "super_l" || keyLower == "super_r" {
			return false, "", errors.New("accel of shortcut which type is wm or metacity can not be set to the Super key")
		}
	}

	var confShortcuts []shortcuts.Shortcut
	for {
		confAccel, _ := m.shortcutManager.FindConflictingAccel(pa)
		if confAccel == nil {
			logger.Debug("confAccel is nil")
			break
		}
		logger.Debug("conflicting accel:", confAccel)
		confShortcut := confAccel.Shortcut
		confShortcuts = append(confShortcuts, confShortcut)
		m.shortcutManager.RemoveShortcutAccel(confShortcut, pa)
		m.emitShortcutSignal(shortcutSignalChanged, confShortcut)
	}
	m.shortcutManager.AddShortcutAccel(shortcut, pa)
	m.emitShortcutSignal(shortcutSignalChanged, shortcut)

	// save accels
	shortcut.SaveAccels()
	for _, confShortcut := range confShortcuts {
		confShortcut.SaveAccels()
	}

	return false, "", nil
}

// Query query shortcut detail info by id and type
func (m *Manager) Query(id string, ty int32) (string, error) {
	shortcut := m.shortcutManager.GetByIdType(id, ty)
	if shortcut == nil {
		return "", ErrShortcutNotFound{id, ty}
	}

	return doMarshal(shortcut)
}

// GrabScreen grab screen for getting the key pressed
func (m *Manager) GrabScreen() error {
	logger.Debug("Manager.GrabScreen")
	return m.doGrabScreen(m.shortcutManager)
}

func (m *Manager) SetNumLockState(state int32) error {
	logger.Debug("SetNumLockState", state)
	return setNumLockState(m.conn, m.keySymbols, NumLockState(state))
}
