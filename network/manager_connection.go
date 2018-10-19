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

package network

import (
	"fmt"
	"sort"

	nmdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.networkmanager"
	"pkg.deepin.io/dde/daemon/network/nm"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	. "pkg.deepin.io/lib/gettext"
)

type connectionSlice []*connection

func (c connectionSlice) Len() int           { return len(c) }
func (c connectionSlice) Less(i, j int) bool { return c[i].Id < c[j].Id }
func (c connectionSlice) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }

type connection struct {
	nmConn   *nmdbus.ConnectionSettings
	connType string

	Path dbus.ObjectPath
	Uuid string
	Id   string

	// if not empty, the connection will only apply to special device,
	// works for wired, wireless, infiniband, wimax devices
	HwAddress     string
	ClonedAddress string

	// works for wireless, olpc-mesh connections
	Ssid string
}

func (m *Manager) initConnectionManage() {
	m.connectionsLock.Lock()
	m.connections = make(map[string]connectionSlice)
	m.connectionsLock.Unlock()

	for _, cpath := range nmGetConnectionList() {
		m.addConnection(cpath)
	}
	nmSettings.ConnectNewConnection(func(cpath dbus.ObjectPath) {
		logger.Info("add connection", cpath)
		m.addConnection(cpath)
	})
	nmSettings.ConnectConnectionRemoved(func(cpath dbus.ObjectPath) {
		logger.Info("remove connection", cpath)
		m.removeConnection(cpath)
	})
}

func (m *Manager) newConnection(cpath dbus.ObjectPath) (conn *connection, err error) {
	conn = &connection{Path: cpath}
	nmConn, err := nmNewSettingsConnection(cpath)
	if err != nil {
		return
	}

	conn.nmConn = nmConn
	conn.updateProps()

	if conn.connType == connectionVpn {
		m.config.addVpnConfig(conn.Uuid)
	}

	// connect signals
	nmConn.InitSignalExt(m.sysSigLoop, true)
	nmConn.ConnectUpdated(func() {
		m.updateConnection(cpath)
	})

	return
}
func (conn *connection) updateProps() {
	cdata, err := conn.nmConn.GetSettings(0)
	if err != nil {
		logger.Error(err)
		return
	}

	conn.Uuid = getSettingConnectionUuid(cdata)
	conn.Id = getSettingConnectionId(cdata)

	switch getSettingConnectionType(cdata) {
	case nm.NM_SETTING_GSM_SETTING_NAME, nm.NM_SETTING_CDMA_SETTING_NAME:
		conn.connType = connectionMobile
	case nm.NM_SETTING_VPN_SETTING_NAME:
		conn.connType = connectionVpn
	default:
		conn.connType = getCustomConnectionType(cdata)
	}

	switch getSettingConnectionType(cdata) {
	case nm.NM_SETTING_WIRED_SETTING_NAME, nm.NM_SETTING_PPPOE_SETTING_NAME:
		if isSettingWiredMacAddressExists(cdata) {
			conn.HwAddress = convertMacAddressToString(getSettingWiredMacAddress(cdata))
		} else {
			conn.HwAddress = ""
		}
		if isSettingWiredClonedMacAddressExists(cdata) {
			conn.ClonedAddress = convertMacAddressToString(getSettingWiredClonedMacAddress(cdata))
		} else {
			conn.ClonedAddress = ""
		}
	case nm.NM_SETTING_WIRELESS_SETTING_NAME:
		conn.Ssid = decodeSsid(getSettingWirelessSsid(cdata))
		if isSettingWirelessMacAddressExists(cdata) {
			conn.HwAddress = convertMacAddressToString(getSettingWirelessMacAddress(cdata))
		} else {
			conn.HwAddress = ""
		}
		if isSettingWirelessChannelExists(cdata) {
			conn.ClonedAddress = convertMacAddressToString(getSettingWirelessClonedMacAddress(cdata))
		} else {
			conn.ClonedAddress = ""
		}
	}
}

func (m *Manager) destroyConnection(conn *connection) {
	m.config.removeConnection(conn.Uuid)
	nmDestroySettingsConnection(conn.nmConn)
}

func (m *Manager) clearConnections() {
	m.connectionsLock.Lock()
	defer m.connectionsLock.Unlock()
	for _, conns := range m.connections {
		for _, conn := range conns {
			m.destroyConnection(conn)
		}
	}
	m.connections = make(map[string]connectionSlice)
	m.updatePropConnections()
}

func (m *Manager) addConnection(cpath dbus.ObjectPath) {
	if m.isConnectionExists(cpath) {
		logger.Warning("connection already exists", cpath)
		return
	}

	m.connectionsLock.Lock()
	defer m.connectionsLock.Unlock()
	conn, err := m.newConnection(cpath)
	if err != nil {
		return
	}
	logger.Infof("add connection %#v", conn)
	switch conn.connType {
	case connectionUnknown:
	default:
		m.connections[conn.connType] = append(m.connections[conn.connType], conn)
		sort.Sort(m.connections[conn.connType])
	}
	m.updatePropConnections()
}

func (m *Manager) removeConnection(cpath dbus.ObjectPath) {
	if !m.isConnectionExists(cpath) {
		logger.Warning("connection not found", cpath)
		return
	}
	connType, i := m.getConnectionIndex(cpath)

	m.connectionsLock.Lock()
	defer m.connectionsLock.Unlock()
	m.connections[connType] = m.doRemoveConnection(m.connections[connType], i)
	m.updatePropConnections()
}

func (m *Manager) doRemoveConnection(conns connectionSlice, i int) connectionSlice {
	logger.Infof("remove connection %#v", conns[i])
	m.destroyConnection(conns[i])
	copy(conns[i:], conns[i+1:])
	conns = conns[:len(conns)-1]
	return conns
}

func (m *Manager) updateConnection(cpath dbus.ObjectPath) {
	if !m.isConnectionExists(cpath) {
		logger.Warning("connection not found", cpath)
		return
	}
	conn := m.getConnection(cpath)

	m.connectionsLock.Lock()
	defer m.connectionsLock.Unlock()
	conn.updateProps()
	logger.Infof("update connection %#v", conn)
	m.updatePropConnections()
}

func (m *Manager) getConnection(cpath dbus.ObjectPath) (conn *connection) {
	connType, i := m.getConnectionIndex(cpath)
	if i < 0 {
		logger.Warning("connection not found", cpath)
		return
	}

	m.connectionsLock.Lock()
	defer m.connectionsLock.Unlock()
	conn = m.connections[connType][i]
	return
}
func (m *Manager) isConnectionExists(cpath dbus.ObjectPath) bool {
	_, i := m.getConnectionIndex(cpath)
	if i >= 0 {
		return true
	}
	return false
}
func (m *Manager) getConnectionIndex(cpath dbus.ObjectPath) (connType string, index int) {
	m.connectionsLock.Lock()
	defer m.connectionsLock.Unlock()
	for t, conns := range m.connections {
		for i, c := range conns {
			if c.Path == cpath {
				return t, i
			}
		}
	}
	return "", -1
}

// GetSupportedConnectionTypes return all supported connection types
func (m *Manager) GetSupportedConnectionTypes() (types []string, err *dbus.Error) {
	return supportedConnectionTypes, nil
}

// TODO: remove, use device.UniqueUuid instead
// GetWiredConnectionUuid return connection uuid for target wired device.
func (m *Manager) GetWiredConnectionUuid(wiredDevPath dbus.ObjectPath) (uuid string, err *dbus.Error) {
	// this interface will be called by front-end always if user try
	// to connect or edit the wired connection, so ensure the
	// connection exists here is a good choice
	m.ensureWiredConnectionExists(wiredDevPath, false)
	uuid = nmGeneralGetDeviceUniqueUuid(wiredDevPath)
	return
}

func (m *Manager) generalEnsureUniqueConnectionExists(devPath dbus.ObjectPath, active bool) (cpath dbus.ObjectPath, exists bool, err error) {
	cpath = "/"
	switch nmGetDeviceType(devPath) {
	case nm.NM_DEVICE_TYPE_ETHERNET:
		cpath, exists, err = m.ensureWiredConnectionExists(devPath, active)
	case nm.NM_DEVICE_TYPE_WIFI: // ignore
	case nm.NM_DEVICE_TYPE_MODEM:
		cpath, exists, err = m.ensureMobileConnectionExists(devPath, active)
	}
	return
}

// ensureWiredConnectionExists will check if wired connection for
// target device exists, if not, create one.
func (m *Manager) ensureWiredConnectionExists(wiredDevPath dbus.ObjectPath, active bool) (cpath dbus.ObjectPath, exists bool, err error) {
	uuid := nmGeneralGetDeviceUniqueUuid(wiredDevPath)
	var id string
	if nmGeneralIsUsbDevice(wiredDevPath) {
		id = nmGeneralGetDeviceDesc(wiredDevPath)
	} else {
		id = Tr("Wired Connection")
	}

	cpath, err = nmGetConnectionByUuid(uuid)
	if err != nil {
		// try get uuid from active or available connection
		logger.Info("-----------Try get uuid, src:", uuid)
		uuid = getWiredDeviceConnectionUuid(wiredDevPath)
		logger.Info("-----------Try get uuid, result:", uuid)
		if uuid != "" {
			cpath, err = nmGetConnectionByUuid(uuid)
		}
	}
	if err != nil {
		// connection not exists, create one
		exists = false
		cpath, err = newWiredConnectionForDevice(id, uuid, wiredDevPath, active)
	} else {
		// connection already exists, reset its name to keep
		// consistent with current system's language
		exists = true
		//nmSetConnectionId(cpath, id)
	}
	return
}

func getWiredDeviceConnectionUuid(wiredDevPath dbus.ObjectPath) string {
	wired, _ := nmNewDevice(wiredDevPath)
	if wired == nil {
		return ""
	}

	apath, _ := wired.ActiveConnection().Get(0)
	if apath != "" && apath != "/" {
		aconn, _ := nmNewActiveConnection(apath)
		if aconn != nil {
			uuid, _ := aconn.Uuid().Get(0)
			return uuid
		}
	}

	list, _ := wired.AvailableConnections().Get(0)
	if len(list) != 0 && list[0] != "/" {
		sconn, _ := nmNewSettingsConnection(list[0])
		settings, _ := sconn.GetSettings(0)
		return settings["connection"]["uuid"].Value().(string)
	}
	return ""
}

// ensureMobileConnectionExists will check if mobile connection for
// target device exists, if not, create one.
func (m *Manager) ensureMobileConnectionExists(modemDevPath dbus.ObjectPath, active bool) (cpath dbus.ObjectPath, exists bool, err error) {
	uuid := nmGeneralGetDeviceUniqueUuid(modemDevPath)
	cpath, err = nmGetConnectionByUuid(uuid)
	if err == nil {
		// connection already exists
		exists = true
		return
	}
	// connection id will be reset when setting up plans, so here just give
	// an optional name like "mobile"
	cpath, err = newMobileConnectionForDevice("mobile", uuid, modemDevPath, active)
	return
}

// ensureWirelessHotspotConnectionExists will check if wireless hotspot connection for
// target device exists, if not, create one.
func (m *Manager) ensureWirelessHotspotConnectionExists(wirelessDevPath dbus.ObjectPath, active bool) (cpath dbus.ObjectPath, exists bool, err error) {
	uuid := nmGeneralGetDeviceUniqueUuid(wirelessDevPath)
	cpath, err = nmGetConnectionByUuid(uuid)
	if err == nil {
		// connection already exists
		exists = true
		return
	}

	// connection not exists, create one
	cpath, err = newWirelessHotspotConnectionForDevice("hotspot", uuid, wirelessDevPath, active)
	return
}

// CreateConnection create a new connection, return ConnectionSession's dbus object path if success.
func (m *Manager) CreateConnection(connType string, devPath dbus.ObjectPath) (sessionPath dbus.ObjectPath,
	busErr *dbus.Error) {
	logger.Debug("CreateConnection", connType, devPath)
	session, err := newConnectionSessionByCreate(connType, devPath, m.service)
	if err != nil {
		logger.Warning(err)
		return "/", dbusutil.ToError(err)
	}
	m.addConnectionSession(session)
	return session.sessionPath, nil
}

// EditConnection open a connection through uuid, return ConnectionSession's dbus object path if success.
func (m *Manager) EditConnection(uuid string, devPath dbus.ObjectPath) (
	sessionPath dbus.ObjectPath, busErr *dbus.Error) {
	session, err := m.editConnection(uuid, devPath)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return session.sessionPath, nil
}

func (m *Manager) editConnection(uuid string, devPath dbus.ObjectPath) (session *ConnectionSession, err error) {
	logger.Debug("EditConnection", uuid, devPath)
	if isNmObjectPathValid(devPath) && nmGeneralGetDeviceUniqueUuid(devPath) == uuid {
		m.generalEnsureUniqueConnectionExists(devPath, false)
	}
	session, err = newConnectionSessionByOpen(uuid, devPath, m.service)
	if err != nil {
		logger.Error(err)
		return
	}
	m.addConnectionSession(session)
	return
}

func (m *Manager) addConnectionSession(session *ConnectionSession) {
	m.connectionSessionsLock.Lock()
	defer m.connectionSessionsLock.Unlock()

	// install dbus session
	err := m.service.Export(session.sessionPath, session)
	if err != nil {
		logger.Error(err)
		return
	}
	m.connectionSessions = append(m.connectionSessions, session)
}
func (m *Manager) removeConnectionSession(session *ConnectionSession) {
	m.connectionSessionsLock.Lock()
	defer m.connectionSessionsLock.Unlock()

	m.service.StopExport(session)

	i := m.getConnectionSessionIndex(session)
	if i < 0 {
		logger.Warning("connection session index is -1", session.sessionPath)
		return
	}

	copy(m.connectionSessions[i:], m.connectionSessions[i+1:])
	newlen := len(m.connectionSessions) - 1
	m.connectionSessions[newlen] = nil
	m.connectionSessions = m.connectionSessions[:newlen]
}
func (m *Manager) getConnectionSessionIndex(session *ConnectionSession) int {
	for i, s := range m.connectionSessions {
		if s.sessionPath == session.sessionPath {
			return i
		}
	}
	return -1
}
func (m *Manager) clearConnectionSessions() {
	m.connectionSessionsLock.Lock()
	defer m.connectionSessionsLock.Unlock()
	for _, session := range m.connectionSessions {
		m.service.StopExport(session)
	}
	m.connectionSessions = nil
}

func (m *Manager) uninstallConnectionSession(uuid string, activating bool) {
	m.connectionSessionsLock.Lock()
	defer m.connectionSessionsLock.Unlock()

	var (
		s   *ConnectionSession
		idx int
	)
	for i, session := range m.connectionSessions {
		if session.Uuid != uuid {
			continue
		}

		s = session
		idx = i
		break
	}

	if s == nil {
		logger.Warning("Failed to uninstall connection session: not found", uuid)
		return
	}

	copy(m.connectionSessions[idx:], m.connectionSessions[idx+1:])
	newlen := len(m.connectionSessions) - 1
	m.connectionSessions[newlen] = nil
	m.connectionSessions = m.connectionSessions[:newlen]

	if activating && s.Type == connectionWirelessHotspot {
		// The device maybe not ready switch from AP mode, so reset managed
		logger.Debug("[uninstallConnectionSession] Will uninstall hotspot connection, reset managed state:", s.devPath)
		m.setDeviceManaged(string(s.devPath), false)
		m.setDeviceManaged(string(s.devPath), true)
	}
	m.service.StopExport(s)
	s = nil
}

// DeleteConnection delete a connection through uuid.
func (m *Manager) DeleteConnection(uuid string) *dbus.Error {
	err := m.deleteConnection(uuid)
	return dbusutil.ToError(err)
}

func (m *Manager) deleteConnection(uuid string) (err error) {
	cpath, err := nmGetConnectionByUuid(uuid)
	if err != nil {
		return
	}

	nmConn, err := nmNewSettingsConnection(cpath)
	if err != nil {
		return
	}

	// uninstall opened connection session
	activating := isConnectionInActivating(uuid)
	defer m.uninstallConnectionSession(uuid, activating)

	return nmConn.Delete(0)
}

func (m *Manager) ActivateConnection(uuid string, devPath dbus.ObjectPath) (
	cpath dbus.ObjectPath, busErr *dbus.Error) {
	cpath, err := m.activateConnection(uuid, devPath)
	busErr = dbusutil.ToError(err)
	return
}

// activateConnection try to activate target connection, if not
// special a valid devPath just left it as "/".
// TODO: return apath instead of cpath
func (m *Manager) activateConnection(uuid string, devPath dbus.ObjectPath) (cpath dbus.ObjectPath, err error) {
	logger.Debugf("ActivateConnection: uuid=%s, devPath=%s", uuid, devPath)
	cpath = "/"
	if devPath == "" {
		err = fmt.Errorf("Device path is empty")
		logger.Warning("ActivateConnection empty device path:", uuid)
		return
	}

	cpath, err = nmGetConnectionByUuid(uuid)
	if err != nil {
		// connection will be activated in generalEnsureUniqueConnectionExists() if not exists
		if devPath != "/" && nmGeneralGetDeviceUniqueUuid(devPath) == uuid {
			cpath, _, err = m.generalEnsureUniqueConnectionExists(devPath, true)
		}
		return
	}
	_, err = nmActivateConnection(cpath, devPath)
	return
}

// DeactivateConnection deactivate a target connection.
func (m *Manager) DeactivateConnection(uuid string) *dbus.Error {
	err := m.deactivateConnection(uuid)
	return dbusutil.ToError(err)
}

func (m *Manager) deactivateConnection(uuid string) (err error) {
	apaths, err := nmGetActiveConnectionByUuid(uuid)
	if err != nil {
		// not found active connection with uuid, ignore error here
		return
	}
	for _, apath := range apaths {
		logger.Debug("DeactivateConnection:", uuid, apath)
		if isConnectionStateInActivating(nmGetActiveConnectionState(apath)) {
			if tmpErr := nmDeactivateConnection(apath); tmpErr != nil {
				err = tmpErr
			}
		}
	}
	return
}

// EnableWirelessHotspotMode activate the device related hotspot
// connection, if the connection not exists will create one.
func (m *Manager) EnableWirelessHotspotMode(devPath dbus.ObjectPath) *dbus.Error {
	err := m.enableWirelessHotSpotMode(devPath)
	return dbusutil.ToError(err)
}

func (m *Manager) enableWirelessHotSpotMode(devPath dbus.ObjectPath) (err error) {
	devType := nmGetDeviceType(devPath)
	if devType != nm.NM_DEVICE_TYPE_WIFI {
		err = fmt.Errorf("not a wireless device %s %d", devPath, devType)
		logger.Error(err)
		return
	}

	cpath, exists, err := m.ensureWirelessHotspotConnectionExists(devPath, true)
	if exists {
		// if the connection not exists, it will be activated when
		// creating, but if already exists, we should activate it
		// manually
		_, err = nmActivateConnection(cpath, devPath)
	}
	return
}

// DisableWirelessHotspotMode will disconnect the device related hotspot connection.
func (m *Manager) DisableWirelessHotspotMode(devPath dbus.ObjectPath) *dbus.Error {
	uuid := nmGeneralGetDeviceUniqueUuid(devPath)
	err := m.deactivateConnection(uuid)
	return dbusutil.ToError(err)
}

// IsWirelessHotspotModeEnabled check if the device related hotspot
// connection activated.
func (m *Manager) IsWirelessHotspotModeEnabled(devPath dbus.ObjectPath) (enabled bool, err *dbus.Error) {
	uuid := nmGeneralGetDeviceUniqueUuid(devPath)
	apaths, _ := nmGetActiveConnectionByUuid(uuid)
	if len(apaths) > 0 {
		// the target hotspot connection is activated
		enabled = true
	}
	return
}

// DisconnectDevice will disconnect all connection in target device,
// DisconnectDevice is different with DeactivateConnection, for
// example if user deactivate current connection for a wireless
// device, NetworkManager will try to activate another access point if
// available then, but if call DisconnectDevice for the device, the
// device will keep disconnected later.
func (m *Manager) DisconnectDevice(devPath dbus.ObjectPath) *dbus.Error {
	err := m.doDisconnectDevice(devPath)
	return dbusutil.ToError(err)
}

func (m *Manager) doDisconnectDevice(devPath dbus.ObjectPath) (err error) {
	nmDev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}

	devState, _ := nmDev.State().Get(0)
	if isDeviceStateInActivating(devState) {
		err = nmDev.Disconnect(0)
		if err != nil {
			logger.Error(err)
		}
	}
	return
}

// IsPasswordValid check if password value for target connection types
// is valid. This will be used by the front-end password input dialog.
func (m *Manager) IsPasswordValid(passType, value string) (ok bool, err *dbus.Error) {
	return isPasswordValid(passType, value), nil
}
