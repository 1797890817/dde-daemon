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
	"sync"
	"time"

	"pkg.deepin.io/lib/dbus1"

	"pkg.deepin.io/dde/daemon/network/proxychains"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	dbusServiceName = "com.deepin.daemon.Network"
	dbusPath        = "/com/deepin/daemon/Network"
	dbusInterface   = "com.deepin.daemon.Network"
)

type connectionData map[string]map[string]dbus.Variant

//go:generate dbusutil-gen -type Manager manager.go

// Manager is the main DBus object for network module.
type Manager struct {
	sysSigLoop *dbusutil.SignalLoop
	service    *dbusutil.Service
	config     *config

	PropsMu sync.RWMutex
	// update by manager.go
	State uint32 // global networking state

	NetworkingEnabled bool `prop:"access:rw"` // airplane mode for NetworkManager
	VpnEnabled        bool `prop:"access:rw"`

	// hidden properties
	wirelessEnabled bool
	wwanEnabled     bool
	wiredEnabled    bool

	// update by manager_devices.go
	devicesLock sync.Mutex
	devices     map[string][]*device
	Devices     string // array of device objects and marshaled by json

	accessPointsLock sync.Mutex
	accessPoints     map[dbus.ObjectPath][]*accessPoint

	// update by manager_connections.go
	connectionsLock sync.Mutex
	connections     map[string]connectionSlice
	Connections     string // array of connection information and marshaled by json

	connectionSessionsLock sync.Mutex
	connectionSessions     []*ConnectionSession

	// update by manager_active.go
	activeConnectionsLock sync.Mutex
	activeConnections     map[dbus.ObjectPath]*activeConnection
	ActiveConnections     string // array of connections that activated and marshaled by json

	agent         *agent
	stateHandler  *stateHandler
	dbusWatcher   *dbusWatcher
	switchHandler *switchHandler

	proxyChainsManager *proxychains.Manager

	signals *struct {
		// NeedSecrets send signal to front-end to pop-up password input
		// dialog to fill the secrets.
		NeedSecrets struct {
			secretsInfoJSON string
		}
		NeedSecretsFinished struct {
			connPath, settingName string
		}
		AccessPointAdded, AccessPointRemoved, AccessPointPropertiesChanged struct {
			devPath, apJSON string
		}
		DeviceEnabled struct {
			devPath string
			enabled bool
		}
	}

	methods *struct {
		ActivateAccessPoint            func() `in:"uuid,apPath,devPath" out:"cPath"`
		ActivateConnection             func() `in:"uuid,devPath" out:"cPath"`
		CancelSecret                   func() `in:"path,settingName"`
		CreateConnection               func() `in:"connType,devPath" out:"sessionPath"`
		CreateConnectionForAccessPoint func() `in:"apPath,devPath" out:"sessionPath"`
		DeactivateConnection           func() `in:"uuid"`
		DeleteConnection               func() `in:"uuid"`
		DisableWirelessHotspotMode     func() `in:"devPath"`
		DisconnectDevice               func() `in:"devPath"`
		EditConnection                 func() `in:"uuid,devPath" out:"sessionPath"`
		EnableDevice                   func() `in:"devPath,enabled"`
		EnableWirelessHotspotMode      func() `in:"devPath"`
		FeedSecret                     func() `in:"path,settingName,keyValue,autoConnect"`
		GetAccessPoints                func() `in:"path" out:"apsJSON"`
		GetActiveConnectionInfo        func() `out:"acInfosJSON"`
		GetAutoProxy                   func() `out:"proxyAuto"`
		GetProxy                       func() `in:"proxyType" out:"host,port"`
		GetProxyIgnoreHosts            func() `out:"ignoreHosts"`
		GetProxyMethod                 func() `out:"proxyMode"`
		GetSupportedConnectionTypes    func() `out:"types"`
		GetWiredConnectionUuid         func() `in:"wiredDevPath" out:"uuid"`
		IsDeviceEnabled                func() `in:"devPath" out:"enabled"`
		IsPasswordValid                func() `in:"passType,value" out:"ok"`
		IsWirelessHotspotModeEnabled   func() `in:"devPath" out:"enabled"`
		ListDeviceConnections          func() `in:"devPath" out:"connections"`
		SetAutoProxy                   func() `in:"proxyAuto"`
		SetDeviceManaged               func() `in:"devPathOrIfc,managed"`
		SetProxy                       func() `in:"proxyType,host,port"`
		SetProxyIgnoreHosts            func() `in:"ignoreHosts"`
		SetProxyMethod                 func() `in:"proxyMode"`
	}
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

// initialize slice code manually to make i18n works
func initSlices() {
	initVirtualSections()
	initProxyGsettings()
	initAvailableValuesSecretFlags()
	initAvailableValuesNmPptpSecretFlags()
	initAvailableValuesNmL2tpSecretFlags()
	initAvailableValuesNmVpncSecretFlags()
	initAvailableValuesNmOpenvpnSecretFlags()
	initAvailableValuesWirelessChannel()
	initAvailableValues8021x()
	initAvailableValuesIp4()
	initAvailableValuesIp6()
	initNmStateReasons()
}

func NewManager(service *dbusutil.Service) (m *Manager) {
	m = &Manager{
		service: service,
	}
	return
}

func (m *Manager) init() {
	logger.Info("initialize network")

	systemBus, err := dbus.SystemBus()
	if err != nil {
		return
	}

	m.sysSigLoop = dbusutil.NewSignalLoop(systemBus, 10)
	m.sysSigLoop.Start()
	m.initDbusObjects()

	disableNotify()
	defer enableNotify()

	m.config = newConfig()
	m.switchHandler = newSwitchHandler(m.config)
	m.dbusWatcher = newDbusWatcher(true)
	m.stateHandler = newStateHandler(m.config, m.sysSigLoop)

	sysService, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Warning(err)
		return
	}

	m.agent = newAgent(sysService)

	// initialize device and connection handlers
	m.initDeviceManage()
	m.initConnectionManage()
	m.initActiveConnectionManage()

	// update property "State"
	nmManager.State().ConnectChanged(func(hasValue bool, value uint32) {
		m.updatePropState()
	})
	m.updatePropState()

	// TODO: notifications issue when resume from suspend

	// connect computer suspend signal
	loginManager.ConnectPrepareForSleep(func(active bool) {
		if active {
			// suspend
			disableNotify()
		} else {
			// restore
			m.switchHandler.init()
			enableNotify()

			m.RequestWirelessScan()
		}
	})
}

func (m *Manager) destroy() {
	logger.Info("destroy network")
	destroyDbusObjects()
	destroyAgent(m.agent)
	destroyStateHandler(m.stateHandler)
	destroyDbusWatcher(m.dbusWatcher)
	m.clearDevices()
	m.clearAccessPoints()
	m.clearConnections()
	m.clearConnectionSessions()
	m.clearActiveConnections()

	// reset dbus properties
	m.setPropNetworkingEnabled(false)
	m.updatePropState()
}

func watchNetworkManagerRestart(m *Manager) {
	dbusDaemon.ConnectNameOwnerChanged(func(name, oldOwner, newOwner string) {
		if name == "org.freedesktop.NetworkManager" {
			// if a new dbus session was installed, the name and newOwner
			// will be no empty, if a dbus session was uninstalled, the
			// name and oldOwner will be not empty
			if len(newOwner) != 0 {
				// network-manager is starting
				logger.Info("network-manager is starting")
				time.Sleep(1 * time.Second)
				m.init()
			} else {
				// network-manager stopped
				logger.Info("network-manager stopped")
				m.destroy()
			}
		}
	})
}
