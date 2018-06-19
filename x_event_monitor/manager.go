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

package x_event_monitor

import (
	"errors"
	"fmt"
	"sync"

	"github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/util/keysyms"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/strv"
	dutils "pkg.deepin.io/lib/utils"
)

const _FullscreenId = "d41d8cd98f00b204e9800998ecf8427e"

var errAreasRegistered = errors.New("the areas has been registered")
var errAreasNotRegistered = errors.New("the areas has not been registered yet")

type coordinateInfo struct {
	areas        []coordinateRange
	moveIntoFlag bool
	motionFlag   bool
	buttonFlag   bool
	keyFlag      bool
}

type coordinateRange struct {
	X1 int32
	Y1 int32
	X2 int32
	Y2 int32
}

type Manager struct {
	xConn      *x.Conn
	keySymbols *keysyms.KeySymbols
	service    *dbusutil.Service
	signals    *struct {
		CancelAllArea struct{}

		CursorInto, CursorOut, CursorMove struct {
			x, y int32
			id   string
		}

		ButtonPress, ButtonRelease struct {
			button, x, y int32
			id           string
		}
		KeyPress, KeyRelease struct {
			key  string
			x, y int32
			id   string
		}
	}

	methods *struct {
		RegisterArea       func() `in:"x1,y1,x2,y2,flag" out:"id"`
		RegisterAreas      func() `in:"areas,flag" out:"id"`
		RegisterFullScreen func() `out:"id"`
		UnregisterArea     func() `in:"id" out:"ok"`
	}

	pidAidsMap      map[uint32]strv.Strv
	idAreaInfoMap   map[string]*coordinateInfo
	idReferCountMap map[string]int32

	mu sync.Mutex
}

func newManager(service *dbusutil.Service) (*Manager, error) {
	xConn, err := x.NewConn()
	if err != nil {
		return nil, err
	}
	keySymbols := keysyms.NewKeySymbols(xConn)

	return &Manager{
		xConn:           xConn,
		keySymbols:      keySymbols,
		service:         service,
		pidAidsMap:      make(map[uint32]strv.Strv),
		idAreaInfoMap:   make(map[string]*coordinateInfo),
		idReferCountMap: make(map[string]int32),
	}, nil
}

func (m *Manager) handleXEvent() {
	eventChan := make(chan x.GenericEvent, 10)
	m.xConn.AddEventChan(eventChan)

	for ev := range eventChan {
		switch ev.GetEventCode() {
		case x.MappingNotifyEventCode:
			logger.Debug("mapping notify event")
			event, _ := x.NewMappingNotifyEvent(ev)
			m.keySymbols.RefreshKeyboardMapping(event)
		}
	}
}

func (m *Manager) handleCursorEvent(x, y int32, press bool) {
	press = !press

	inList, outList := m.getIdList(x, y)
	for _, id := range inList {
		array, ok := m.idAreaInfoMap[id]
		if !ok {
			continue
		}

		/* moveIntoFlag == true : mouse move in area */
		if !array.moveIntoFlag {
			if press {
				m.service.Emit(m, "CursorInto", x, y, id)
				array.moveIntoFlag = true
			}
		}

		if array.motionFlag {
			m.service.Emit(m, "CursorMove", x, y, id)
		}
	}
	for _, id := range outList {
		array, ok := m.idAreaInfoMap[id]
		if !ok {
			continue
		}

		/* moveIntoFlag == false : mouse move out area */
		if array.moveIntoFlag {
			m.service.Emit(m, "CursorOut", x, y, id)
			array.moveIntoFlag = false
		}
	}

	_, ok := m.idReferCountMap[_FullscreenId]
	if !ok {
		return
	}

	m.service.Emit(m, "CursorMove", x, y, _FullscreenId)
}

func (m *Manager) handleButtonEvent(button int32, press bool, x, y int32) {

	list, _ := m.getIdList(x, y)
	for _, id := range list {
		array, ok := m.idAreaInfoMap[id]
		if !ok || !array.buttonFlag {
			continue
		}

		if press {
			m.service.Emit(m, "ButtonPress", button, x, y, id)
		} else {
			m.service.Emit(m, "ButtonRelease", button, x, y, id)
		}
	}

	_, ok := m.idReferCountMap[_FullscreenId]
	if !ok {
		return
	}

	if press {
		m.service.Emit(m, "ButtonPress", button, x, y, _FullscreenId)
	} else {
		m.service.Emit(m, "ButtonRelease", button, x, y, _FullscreenId)
	}
}

func (m *Manager) keyCode2Str(key int32) string {
	str, _ := m.keySymbols.LookupString(x.Keycode(key), 0)
	return str
}

func (m *Manager) handleKeyboardEvent(code int32, press bool, x, y int32) {
	logger.Debug("call handleKeyboardEvent", code, press, x, y)
	list, _ := m.getIdList(x, y)
	for _, id := range list {
		array, ok := m.idAreaInfoMap[id]
		if !ok || !array.keyFlag {
			continue
		}

		if press {
			m.service.Emit(m, "KeyPress", m.keyCode2Str(code), x, y, id)
		} else {
			m.service.Emit(m, "KeyRelease", m.keyCode2Str(code), x, y, id)
		}
	}

	_, ok := m.idReferCountMap[_FullscreenId]
	if !ok {
		return
	}

	if press {
		m.service.Emit(m, "KeyPress", m.keyCode2Str(code), x, y, _FullscreenId)
	} else {
		m.service.Emit(m, "KeyRelease", m.keyCode2Str(code), x, y, _FullscreenId)
	}
}

func (m *Manager) cancelAllRegisterArea() {
	m.idAreaInfoMap = make(map[string]*coordinateInfo)
	m.idReferCountMap = make(map[string]int32)

	m.service.Emit(m, "CancelAllArea")
}

func (m *Manager) isPidAreaRegistered(pid uint32, areasId string) bool {
	areasIds := m.pidAidsMap[pid]
	return areasIds.Contains(areasId)
}

func (m *Manager) registerPidArea(pid uint32, areasId string) {
	areasIds := m.pidAidsMap[pid]
	areasIds, _ = areasIds.Add(areasId)
	m.pidAidsMap[pid] = areasIds
}

func (m *Manager) unregisterPidArea(pid uint32, areasId string) {
	areasIds := m.pidAidsMap[pid]
	areasIds, _ = areasIds.Delete(areasId)
	if len(areasIds) > 0 {
		m.pidAidsMap[pid] = areasIds
	} else {
		delete(m.pidAidsMap, pid)
	}
}

func (m *Manager) RegisterArea(sender dbus.Sender, x1, y1, x2, y2, flag int32) (string, *dbus.Error) {
	return m.RegisterAreas(sender,
		[]coordinateRange{coordinateRange{x1, y1, x2, y2}},
		flag)
}

func (m *Manager) RegisterAreas(sender dbus.Sender, areas []coordinateRange, flag int32) (id string, busErr *dbus.Error) {
	md5Str, ok := m.sumAreasMd5(areas, flag)
	if !ok {
		busErr = dbusutil.ToError(fmt.Errorf("sumAreasMd5 failed: %v", areas))
		return
	}
	id = md5Str
	pid, err := m.service.GetConnPID(string(sender))
	if err != nil {
		busErr = dbusutil.ToError(err)
		return
	}
	logger.Debugf("RegisterAreas id %q pid %d", id, pid)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isPidAreaRegistered(pid, id) {
		logger.Warningf("RegisterAreas id %q pid %d failed: %v", id, pid, errAreasRegistered)
		return "", dbusutil.ToError(errAreasRegistered)
	}
	m.registerPidArea(pid, id)

	_, ok = m.idReferCountMap[id]
	if ok {
		m.idReferCountMap[id] += 1
		return id, nil
	}

	info := &coordinateInfo{}
	info.areas = areas
	info.moveIntoFlag = false
	info.buttonFlag = hasButtonFlag(flag)
	info.keyFlag = hasKeyFlag(flag)
	info.motionFlag = hasMotionFlag(flag)

	m.idAreaInfoMap[id] = info
	m.idReferCountMap[id] = 1

	return id, nil
}

func (m *Manager) RegisterFullScreen(sender dbus.Sender) (id string, busErr *dbus.Error) {
	pid, err := m.service.GetConnPID(string(sender))
	if err != nil {
		busErr = dbusutil.ToError(err)
		return
	}
	logger.Debugf("RegisterFullScreen pid %d", pid)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isPidAreaRegistered(pid, _FullscreenId) {
		logger.Warningf("RegisterFullScreen pid %d failed: %v", pid, errAreasRegistered)
		return "", dbusutil.ToError(errAreasRegistered)
	}

	_, ok := m.idReferCountMap[_FullscreenId]
	if !ok {
		m.idReferCountMap[_FullscreenId] = 1
	} else {
		m.idReferCountMap[_FullscreenId] += 1
	}
	m.registerPidArea(pid, _FullscreenId)
	return _FullscreenId, nil
}

func (m *Manager) UnregisterArea(sender dbus.Sender, id string) (bool, *dbus.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pid, err := m.service.GetConnPID(string(sender))
	if err != nil {
		return false, dbusutil.ToError(err)
	}
	logger.Debugf("UnregisterArea id %q pid %d", id, pid)
	if !m.isPidAreaRegistered(pid, id) {
		logger.Warningf("UnregisterArea id %q pid %d failed: %v", id, pid, errAreasNotRegistered)
		return false, nil
	}

	m.unregisterPidArea(pid, id)

	_, ok := m.idReferCountMap[id]
	if !ok {
		logger.Warningf("not found key %q in idReferCountMap", id)
		return false, nil
	}

	m.idReferCountMap[id] -= 1
	if m.idReferCountMap[id] == 0 {
		delete(m.idReferCountMap, id)
		delete(m.idAreaInfoMap, id)
	}
	logger.Debugf("area %q unregistered by pid %d", id, pid)
	return true, nil
}

func (m *Manager) getIdList(x, y int32) ([]string, []string) {
	inList := []string{}
	outList := []string{}

	for id, array := range m.idAreaInfoMap {
		inFlag := false
		for _, area := range array.areas {
			if isInArea(x, y, area) {
				inFlag = true
				if !isInIdList(id, inList) {
					inList = append(inList, id)
				}
			}
		}
		if !inFlag {
			if !isInIdList(id, outList) {
				outList = append(outList, id)
			}
		}
	}

	return inList, outList
}

func (m *Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) sumAreasMd5(areas []coordinateRange, flag int32) (md5Str string, ok bool) {
	if len(areas) < 1 {
		return
	}

	content := ""
	for _, area := range areas {
		if len(content) > 1 {
			content += "-"
		}
		content += fmt.Sprintf("%v-%v-%v-%v", area.X1, area.Y1, area.X2, area.Y2)
	}
	content += fmt.Sprintf("-%v", flag)

	logger.Debug("areas content:", content)
	md5Str, ok = dutils.SumStrMd5(content)

	return
}
