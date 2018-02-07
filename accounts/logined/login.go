/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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

package logined

import (
	"dbus/org/freedesktop/login1"
	"encoding/json"
	"fmt"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/log"
	"sync"
)

// Manager manager logined user list
type Manager struct {
	core   *login1.Manager
	logger *log.Logger

	userSessions map[uint32]SessionInfos
	locker       sync.Mutex

	UserList string
}

const (
	dbusLogin1Dest = "org.freedesktop.login1"
	dbusLogin1Path = "/org/freedesktop/login1"
)

// Register register and install loginedManager on dbus
func Register(logger *log.Logger) (*Manager, error) {
	core, err := login1.NewManager(dbusLogin1Dest, dbusLogin1Path)
	if err != nil {
		return nil, err
	}

	var m = &Manager{
		core:         core,
		logger:       logger,
		userSessions: make(map[uint32]SessionInfos),
	}

	go m.init()
	m.handleChanged()
	return m, nil
}

// Unregister destroy and free Manager object
func Unregister(m *Manager) {
	if m == nil {
		return
	}

	if m.core != nil {
		login1.DestroyManager(m.core)
	}

	if m.userSessions != nil {
		m.userSessions = nil
	}

	m = nil
}

func (m *Manager) init() {
	// the result struct: {id, uid, username, seat, path}
	list, err := m.core.ListSessions()
	if err != nil {
		m.logger.Warning("Failed to list sessions:", err)
		return
	}

	for _, value := range list {
		if len(value) != 5 {
			continue
		}

		m.addSession(value[4].(dbus.ObjectPath))
	}
	m.setPropUserList()
}

func (m *Manager) handleChanged() {
	m.core.ConnectSessionNew(func(id string, sessionPath dbus.ObjectPath) {
		m.logger.Debug("[Event] session new:", id, sessionPath)
		added := m.addSession(sessionPath)
		if added {
			m.setPropUserList()
		}
	})
	m.core.ConnectSessionRemoved(func(id string, sessionPath dbus.ObjectPath) {
		m.logger.Debug("[Event] session remove:", id, sessionPath)
		deleted := m.deleteSession(sessionPath)
		if deleted {
			m.setPropUserList()
		}
	})
}

func (m *Manager) addSession(sessionPath dbus.ObjectPath) bool {
	m.logger.Debug("Create user session for:", sessionPath)
	info, err := newSessionInfo(sessionPath)
	if err != nil {
		m.logger.Warning("Failed to add session:", sessionPath, err)
		return false
	}

	m.locker.Lock()
	defer m.locker.Unlock()
	infos, ok := m.userSessions[info.Uid]
	if !ok {
		m.userSessions[info.Uid] = SessionInfos{info}
		return true
	}

	var added = false
	infos, added = infos.Add(info)
	m.userSessions[info.Uid] = infos
	return added
}

func (m *Manager) deleteSession(sessionPath dbus.ObjectPath) bool {
	m.logger.Debug("Delete user session for:", sessionPath)
	m.locker.Lock()
	defer m.locker.Unlock()
	var deleted = false
	for uid, infos := range m.userSessions {
		tmp, ok := infos.Delete(sessionPath)
		if !ok {
			continue
		}
		deleted = true
		if len(tmp) == 0 {
			delete(m.userSessions, uid)
		} else {
			m.userSessions[uid] = tmp
		}
		break
	}
	return deleted
}

func (m *Manager) setPropUserList() {
	m.locker.Lock()
	defer m.locker.Unlock()

	if len(m.userSessions) == 0 {
		return
	}

	data := m.marshalUserSessions()
	if m.UserList == string(data) {
		return
	}
	m.UserList = string(data)
	dbus.NotifyChange(m, "UserList")
}

func (m *Manager) marshalUserSessions() string {
	if len(m.userSessions) == 0 {
		return ""
	}

	var ret = "{"
	for k, v := range m.userSessions {
		data, err := json.Marshal(v)
		if err != nil {
			m.logger.Warning("Failed to marshal:", v, err)
			continue
		}
		ret += fmt.Sprintf("\"%v\":%s,", k, string(data))
	}

	v := []byte(ret)
	v[len(v)-1] = '}'
	return string(v)
}

// GetDBusInfo dbus session interface
func (m *Manager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       "com.deepin.daemon.Accounts",
		ObjectPath: "/com/deepin/daemon/Logined",
		Interface:  "com.deepin.daemon.Logined",
	}
}
