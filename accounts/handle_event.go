/**
 * Copyright (c) 2011 ~ 2015 Deepin, Inc.
 *               2013 ~ 2015 jouyouyun
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

package accounts

import (
	"github.com/howeyc/fsnotify"
	"pkg.deepin.io/dde/daemon/accounts/users"
	"pkg.deepin.io/lib/dbus"
	"strings"
	"time"
)

const (
	userFilePasswd = "/etc/passwd"
	userFileGroup  = "/etc/group"
	userFileShadow = "/etc/shadow"

	lightdmConfig = "/etc/lightdm/lightdm.conf"
	kdmConfig     = "/usr/share/config/kdm/kdmrc"
	gdmConfig     = "/etc/gdm/custom.conf"

	permModeDir = 0755
)

const (
	maxDuration   = time.Second*2 + time.Second/2
	deltaDuration = time.Millisecond * 500
)

const (
	userListNotChange int = iota + 1
	userListAdded
	userListDeleted
)

func (m *Manager) getWatchFiles() []string {
	var list []string
	dmConfig, err := users.GetDMConfig()
	if err == nil {
		list = append(list, dmConfig)
	}

	list = append(list, []string{userFilePasswd, userFileGroup, userFileShadow}...)
	return list
}

func (m *Manager) handleFileChanged(ev *fsnotify.FileEvent) {
	if ev == nil {
		return
	}

	logger.Debug("File changed:", ev)
	switch {
	case strings.Contains(ev.Name, userFilePasswd):
		m.handleUserFileChanged(ev, m.handleFilePasswdChanged)
	case strings.Contains(ev.Name, userFileGroup):
		m.handleUserFileChanged(ev, m.handleFileGroupChanged)
	case strings.Contains(ev.Name, userFileShadow):
		m.handleUserFileChanged(ev, m.handleFileShadowChanged)
	case strings.Contains(ev.Name, lightdmConfig),
		strings.Contains(ev.Name, kdmConfig),
		strings.Contains(ev.Name, gdmConfig):
		m.handleUserFileChanged(ev, m.handleDMConfigChanged)
	}
}

func (m *Manager) handleUserFileChanged(ev *fsnotify.FileEvent, handler func()) {
	if !ev.IsDelete() || handler == nil {
		return
	}

	m.watcher.ResetFileListWatch()
	<-time.After(time.Millisecond * 500)
	handler()
}

func (m *Manager) handleFilePasswdChanged() {
	waitDuration := time.Second * 0
	for waitDuration < maxDuration {
		if m.refreshUserList() {
			break
		}

		waitDuration += deltaDuration
		<-time.After(waitDuration)
	}
}

func (m *Manager) handleFileGroupChanged() {
	m.mapLocker.Lock()
	defer m.mapLocker.Unlock()
	for _, u := range m.usersMap {
		u.updatePropAccountType()
	}
}

func (m *Manager) handleFileShadowChanged() {
	//Update the property 'Locked'
	m.mapLocker.Lock()
	defer m.mapLocker.Unlock()
	for _, u := range m.usersMap {
		u.updatePropLocked()
	}
}

func (m *Manager) handleDMConfigChanged() {
	for _, u := range m.usersMap {
		u.setPropBool(&u.AutomaticLogin, "AutomaticLogin",
			users.IsAutoLoginUser(u.UserName))
	}
}

func (m *Manager) refreshUserList() bool {
	var freshed bool
	ret, status := compareUserList(m.UserList, getUserPaths())
	switch status {
	case userListAdded:
		freshed = true
		m.handleUserAdded(ret)
	case userListDeleted:
		freshed = true
		m.handleUserDeleted(ret)
	}
	return freshed
}

func (m *Manager) handleUserAdded(list []string) {
	var paths = m.UserList
	for _, p := range list {
		err := m.installUserByPath(p)
		if err != nil {
			logger.Errorf("Install user '%s' failed: %v", p, err)
			continue
		}

		paths = append(paths, p)
		dbus.Emit(m, "UserAdded", p)
		m.copyUserDatas(p)
	}

	m.setPropUserList(paths)
}

func (m *Manager) handleUserDeleted(list []string) {
	var paths = m.UserList
	for _, p := range list {
		m.uninstallUser(p)
		paths = deleteStrFromList(p, paths)
		dbus.Emit(m, "UserDeleted", p)
	}

	m.setPropUserList(paths)
}

func compareUserList(oldList, newList []string) ([]string, int) {
	var (
		ret    []string
		oldLen = len(oldList)
		newLen = len(newList)
	)

	if oldLen < newLen {
		for _, v := range newList {
			if isStrInArray(v, oldList) {
				continue
			}
			ret = append(ret, v)
		}
		return ret, userListAdded
	} else if oldLen > newLen {
		for _, v := range oldList {
			if isStrInArray(v, newList) {
				continue
			}
			ret = append(ret, v)
		}
		return ret, userListDeleted
	}

	return ret, userListNotChange
}

func deleteStrFromList(str string, list []string) []string {
	var ret []string
	for _, v := range list {
		if v == str {
			continue
		}
		ret = append(ret, v)
	}

	return ret
}
