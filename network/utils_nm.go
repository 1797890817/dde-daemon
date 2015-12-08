/**
 * Copyright (c) 2014 Deepin, Inc.
 *               2014 Xu FaSheng
 *
 * Author:      Xu FaSheng <fasheng.xu@gmail.com>
 * Maintainer:  Xu FaSheng <fasheng.xu@gmail.com>
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

package network

import (
	nm "dbus/org/freedesktop/networkmanager"
	"fmt"
	"pkg.deepin.io/lib/dbus"
	. "pkg.deepin.io/lib/gettext"
	"sort"
	"strings"
)

// Wrapper NetworkManger dbus methods to hide
// "dbus/org/freedesktop/networkmanager" details for other source
// files.

// Custom device state reasons
const (
	CUSTOM_NM_DEVICE_STATE_REASON_CABLE_UNPLUGGED = iota + 1000
	CUSTOM_NM_DEVICE_STATE_REASON_WIRELESS_DISABLED
	CUSTOM_NM_DEVICE_STATE_REASON_MODEM_NO_SIGNAL
	CUSTOM_NM_DEVICE_STATE_REASON_MODEM_WRONG_PLAN
)

// Map IP4/IP6 setting keys to compatible with network-manager 1.0+
const (
	NM_SETTING_IP4_CONFIG_METHOD             = NM_SETTING_IP_CONFIG_METHOD
	NM_SETTING_IP4_CONFIG_DNS                = NM_SETTING_IP_CONFIG_DNS
	NM_SETTING_IP4_CONFIG_DNS_SEARCH         = NM_SETTING_IP_CONFIG_DNS_SEARCH
	NM_SETTING_IP4_CONFIG_ADDRESSES          = NM_SETTING_IP_CONFIG_ADDRESSES
	NM_SETTING_IP4_CONFIG_GATEWAY            = NM_SETTING_IP_CONFIG_GATEWAY
	NM_SETTING_IP4_CONFIG_ROUTES             = NM_SETTING_IP_CONFIG_ROUTES
	NM_SETTING_IP4_CONFIG_ROUTE_METRIC       = NM_SETTING_IP_CONFIG_ROUTE_METRIC
	NM_SETTING_IP4_CONFIG_IGNORE_AUTO_ROUTES = NM_SETTING_IP_CONFIG_IGNORE_AUTO_ROUTES
	NM_SETTING_IP4_CONFIG_IGNORE_AUTO_DNS    = NM_SETTING_IP_CONFIG_IGNORE_AUTO_DNS
	NM_SETTING_IP4_CONFIG_DHCP_HOSTNAME      = NM_SETTING_IP_CONFIG_DHCP_HOSTNAME
	NM_SETTING_IP4_CONFIG_DHCP_SEND_HOSTNAME = NM_SETTING_IP_CONFIG_DHCP_SEND_HOSTNAME
	NM_SETTING_IP4_CONFIG_NEVER_DEFAULT      = NM_SETTING_IP_CONFIG_NEVER_DEFAULT
	NM_SETTING_IP4_CONFIG_MAY_FAIL           = NM_SETTING_IP_CONFIG_MAY_FAIL
)
const (
	NM_SETTING_IP6_CONFIG_METHOD             = NM_SETTING_IP_CONFIG_METHOD
	NM_SETTING_IP6_CONFIG_DNS                = NM_SETTING_IP_CONFIG_DNS
	NM_SETTING_IP6_CONFIG_DNS_SEARCH         = NM_SETTING_IP_CONFIG_DNS_SEARCH
	NM_SETTING_IP6_CONFIG_ADDRESSES          = NM_SETTING_IP_CONFIG_ADDRESSES
	NM_SETTING_IP6_CONFIG_GATEWAY            = NM_SETTING_IP_CONFIG_GATEWAY
	NM_SETTING_IP6_CONFIG_ROUTES             = NM_SETTING_IP_CONFIG_ROUTES
	NM_SETTING_IP6_CONFIG_ROUTE_METRIC       = NM_SETTING_IP_CONFIG_ROUTE_METRIC
	NM_SETTING_IP6_CONFIG_IGNORE_AUTO_ROUTES = NM_SETTING_IP_CONFIG_IGNORE_AUTO_ROUTES
	NM_SETTING_IP6_CONFIG_IGNORE_AUTO_DNS    = NM_SETTING_IP_CONFIG_IGNORE_AUTO_DNS
	NM_SETTING_IP6_CONFIG_DHCP_HOSTNAME      = NM_SETTING_IP_CONFIG_DHCP_HOSTNAME
	NM_SETTING_IP6_CONFIG_DHCP_SEND_HOSTNAME = NM_SETTING_IP_CONFIG_DHCP_SEND_HOSTNAME
	NM_SETTING_IP6_CONFIG_NEVER_DEFAULT      = NM_SETTING_IP_CONFIG_NEVER_DEFAULT
	NM_SETTING_IP6_CONFIG_MAY_FAIL           = NM_SETTING_IP_CONFIG_MAY_FAIL
)

// Helper functions
func isNmObjectPathValid(p dbus.ObjectPath) bool {
	str := string(p)
	if len(str) == 0 || str == "/" {
		return false
	}
	return true
}

func isDeviceTypeValid(devType uint32) bool {
	switch devType {
	case NM_DEVICE_TYPE_GENERIC, NM_DEVICE_TYPE_UNKNOWN, NM_DEVICE_TYPE_BT:
		return false
	}
	return true
}

// check current device state
func isDeviceStateManaged(state uint32) bool {
	if state > NM_DEVICE_STATE_UNMANAGED {
		return true
	}
	return false
}
func isDeviceStateAvailable(state uint32) bool {
	if state > NM_DEVICE_STATE_UNAVAILABLE {
		return true
	}
	return false
}
func isDeviceStateActivated(state uint32) bool {
	if state == NM_DEVICE_STATE_ACTIVATED {
		return true
	}
	return false
}
func isDeviceStateInActivating(state uint32) bool {
	if state >= NM_DEVICE_STATE_PREPARE && state <= NM_DEVICE_STATE_ACTIVATED {
		return true
	}
	return false
}

func isDeviceStateReasonInvalid(reason uint32) bool {
	switch reason {
	case NM_DEVICE_STATE_REASON_UNKNOWN, NM_DEVICE_STATE_REASON_NONE:
		return true
	}
	return false
}

// check if connection activating or activated
func isConnectionStateInActivating(state uint32) bool {
	if state == NM_ACTIVE_CONNECTION_STATE_ACTIVATING ||
		state == NM_ACTIVE_CONNECTION_STATE_ACTIVATED {
		return true
	}
	return false
}
func isConnectionStateActivated(state uint32) bool {
	if state == NM_ACTIVE_CONNECTION_STATE_ACTIVATED {
		return true
	}
	return false
}
func isConnectionStateInDeactivating(state uint32) bool {
	if state == NM_ACTIVE_CONNECTION_STATE_DEACTIVATING ||
		state == NM_ACTIVE_CONNECTION_STATE_DEACTIVATED {
		return true
	}
	return false
}
func isConnectionStateDeactivate(state uint32) bool {
	if state == NM_ACTIVE_CONNECTION_STATE_DEACTIVATED {
		return true
	}
	return false
}

// check if vpn connection activating or activated
func isVpnConnectionStateInActivating(state uint32) bool {
	if state >= NM_VPN_CONNECTION_STATE_PREPARE &&
		state <= NM_VPN_CONNECTION_STATE_ACTIVATED {
		return true
	}
	return false
}
func isVpnConnectionStateActivated(state uint32) bool {
	if state == NM_VPN_CONNECTION_STATE_ACTIVATED {
		return true
	}
	return false
}
func isVpnConnectionStateDeactivate(state uint32) bool {
	if state == NM_VPN_CONNECTION_STATE_DISCONNECTED {
		return true
	}
	return false
}
func isVpnConnectionStateFailed(state uint32) bool {
	if state == NM_VPN_CONNECTION_STATE_FAILED {
		return true
	}
	return false
}

var availableValuesSettingSecretFlags []kvalue

func initAvailableValuesSecretFlags() {
	availableValuesSettingSecretFlags = []kvalue{
		kvalue{NM_SETTING_SECRET_FLAG_NONE, Tr("Saved")}, // system saved
		// kvalue{NM_SETTING_SECRET_FLAG_AGENT_OWNED, Tr("Saved")},
		kvalue{NM_SETTING_SECRET_FLAG_NOT_SAVED, Tr("Always Ask")},
		kvalue{NM_SETTING_SECRET_FLAG_NOT_REQUIRED, Tr("Not Required")},
	}
}

func isSettingRequireSecret(flag uint32) bool {
	if flag == NM_SETTING_SECRET_FLAG_NONE || flag == NM_SETTING_SECRET_FLAG_AGENT_OWNED {
		return true
	}
	return false
}

// General function wrappers for network manager
func nmGeneralGetAllDeviceHwAddr(devType uint32) (allHwAddr map[string]string) {
	allHwAddr = make(map[string]string)
	for _, devPath := range nmGetDevices() {
		if dev, err := nmNewDevice(devPath); err == nil && dev.DeviceType.Get() == devType {
			hwAddr, err := nmGeneralGetDeviceHwAddr(devPath)
			if err == nil {
				allHwAddr[dev.Interface.Get()] = hwAddr
			}
			nm.DestroyDevice(dev)
		}
	}
	return
}
func nmGeneralGetDeviceHwAddr(devPath dbus.ObjectPath) (hwAddr string, err error) {
	hwAddr = "00:00:00:00:00:00"
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}
	defer nm.DestroyDevice(dev)

	devType := dev.DeviceType.Get()
	switch devType {
	case NM_DEVICE_TYPE_ETHERNET:
		devWired, _ := nmNewDeviceWired(devPath)
		hwAddr = devWired.PermHwAddress.Get()
		nm.DestroyDeviceWired(devWired)
	case NM_DEVICE_TYPE_WIFI:
		devWireless, _ := nmNewDeviceWireless(devPath)
		hwAddr = devWireless.PermHwAddress.Get()
		nm.DestroyDeviceWireless(devWireless)
	case NM_DEVICE_TYPE_BT:
		devBluetooth, _ := nmNewDeviceBluetooth(devPath)
		hwAddr = devBluetooth.HwAddress.Get()
		nm.DestroyDeviceBluetooth(devBluetooth)
	case NM_DEVICE_TYPE_OLPC_MESH:
		devOlpcMesh, _ := nmNewDeviceOlpcMesh(devPath)
		hwAddr = devOlpcMesh.HwAddress.Get()
		nm.DestroyDeviceOlpcMesh(devOlpcMesh)
	case NM_DEVICE_TYPE_WIMAX:
		devWiMax, _ := nmNewDeviceWiMax(devPath)
		hwAddr = devWiMax.HwAddress.Get()
		nm.DestroyDeviceWiMax(devWiMax)
	case NM_DEVICE_TYPE_INFINIBAND:
		devInfiniband, _ := nmNewDeviceInfiniband(devPath)
		hwAddr = devInfiniband.HwAddress.Get()
		nm.DestroyDeviceInfiniband(devInfiniband)
	case NM_DEVICE_TYPE_BOND:
		devBond, _ := nmNewDeviceBond(devPath)
		hwAddr = devBond.HwAddress.Get()
		nm.DestroyDeviceBond(devBond)
	case NM_DEVICE_TYPE_BRIDGE:
		devBridge, _ := nmNewDeviceBridge(devPath)
		hwAddr = devBridge.HwAddress.Get()
		nm.DestroyDeviceBridge(devBridge)
	case NM_DEVICE_TYPE_VLAN:
		devVlan, _ := nmNewDeviceVlan(devPath)
		hwAddr = devVlan.HwAddress.Get()
		nm.DestroyDeviceVlan(devVlan)
	case NM_DEVICE_TYPE_GENERIC:
		devGeneric, _ := nmNewDeviceGeneric(devPath)
		hwAddr = devGeneric.HwAddress.Get()
		nm.DestroyDeviceGeneric(devGeneric)
	case NM_DEVICE_TYPE_TEAM:
		devTeam, _ := nmNewDeviceTeam(devPath)
		hwAddr = devTeam.HwAddress.Get()
		nm.DestroyDeviceTeam(devTeam)
	case NM_DEVICE_TYPE_MODEM, NM_DEVICE_TYPE_ADSL:
		// there is no hardware address for such devices
		err = fmt.Errorf("there is no hardware address for device modem and adsl")
	default:
		err = fmt.Errorf("unknown device type %d", devType)
		logger.Error(err)
	}
	hwAddr = strings.ToUpper(hwAddr)
	return
}

func nmGetDeviceIdentifiers() (devIds []string) {
	for _, devPath := range nmGetDevices() {
		id, _ := nmGeneralGetDeviceIdentifier(devPath)
		devIds = append(devIds, id)
	}
	return
}
func nmGeneralGetDeviceIdentifier(devPath dbus.ObjectPath) (devId string, err error) {
	// get device unique identifier, use hardware address if exists
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}
	defer nm.DestroyDevice(dev)

	devType := dev.DeviceType.Get()
	switch devType {
	case NM_DEVICE_TYPE_MODEM:
		modemPath := dev.Udi.Get()
		devId, err = mmGetModemDeviceIdentifier(dbus.ObjectPath(modemPath))
	case NM_DEVICE_TYPE_ADSL:
		err = fmt.Errorf("could not get adsl device identifier now")
		logger.Error(err)
	default:
		devId, err = nmGeneralGetDeviceHwAddr(devPath)
	}
	return
}

// return special unique connection uuid for device, etc wired device
// connection
func nmGeneralGetDeviceUniqueUuid(devPath dbus.ObjectPath) (uuid string) {
	devId, err := nmGeneralGetDeviceIdentifier(devPath)
	if err != nil {
		return
	}
	return strToUuid(devId)
}

// get device network speed (Mb/s)
func nmGeneralGetDeviceSpeed(devPath dbus.ObjectPath) (speedStr string) {
	speed := uint32(0)
	speedStr = Tr("Unknown")
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}
	defer nm.DestroyDevice(dev)

	switch t := dev.DeviceType.Get(); t {
	case NM_DEVICE_TYPE_ETHERNET:
		devWired, _ := nmNewDeviceWired(devPath)
		speed = devWired.Speed.Get()
		nm.DestroyDeviceWired(devWired)
	case NM_DEVICE_TYPE_WIFI:
		devWireless, _ := nmNewDeviceWireless(devPath)
		speed = devWireless.Bitrate.Get() / 1024
		nm.DestroyDeviceWireless(devWireless)
	case NM_DEVICE_TYPE_MODEM:
		// TODO: getting device speed for modem device
	case NM_DEVICE_TYPE_GENERIC:
		// ignore speed
	default:
		err = fmt.Errorf("not support to get device speedStr for device type %d", t)
		logger.Error(err)
	}
	if speed != 0 {
		speedStr = fmt.Sprintf("%d Mb/s", speed)
	}
	return
}

func nmGeneralIsDeviceManaged(devPath dbus.ObjectPath) bool {
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return false
	}
	defer nm.DestroyDevice(dev)

	if !isDeviceStateManaged(dev.State.Get()) {
		return false
	}
	devType := dev.DeviceType.Get()
	switch devType {
	case NM_DEVICE_TYPE_WIFI:
		if !nmGetWirelessHardwareEnabled() {
			return false
		}
	}
	return true
}

func nmGeneralGetDeviceSysPath(devPath dbus.ObjectPath) (sysPath string, err error) {
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}
	defer nm.DestroyDevice(dev)

	switch dev.DeviceType.Get() {
	case NM_DEVICE_TYPE_MODEM:
		sysPath, _ = mmGetModemDeviceSysPath(dbus.ObjectPath(dev.Udi.Get()))
	default:
		sysPath = dev.Udi.Get()
	}
	return
}

func nmGeneralGetDeviceVendor(devPath dbus.ObjectPath) (vendor string) {
	sysPath, err := nmGeneralGetDeviceSysPath(devPath)
	if err != nil {
		return
	}
	vendor = udevGetDeviceVendor(sysPath)
	return
}

func nmGeneralIsUsbDevice(devPath dbus.ObjectPath) bool {
	sysPath, err := nmGeneralGetDeviceSysPath(devPath)
	if err != nil {
		return false
	}
	return udevIsUsbDevice(sysPath)
}

func nmGeneralGetConnectionAutoconnect(cpath dbus.ObjectPath) (autoConnect bool) {
	switch nmGetConnectionType(cpath) {
	case NM_SETTING_VPN_SETTING_NAME:
		uuid, _ := nmGetConnectionUuid(cpath)
		autoConnect = manager.config.isVpnConnectionAutoConnect(uuid)
	default:
		autoConnect = nmGetConnectionAutoconnect(cpath)
	}
	return
}
func nmGeneralSetConnectionAutoconnect(cpath dbus.ObjectPath, autoConnect bool) {
	switch nmGetConnectionType(cpath) {
	case NM_SETTING_VPN_SETTING_NAME:
		uuid, _ := nmGetConnectionUuid(cpath)
		manager.config.setVpnConnectionAutoConnect(uuid, autoConnect)
	default:
		nmSetConnectionAutoconnect(cpath, autoConnect)
	}
}

// New network manager objects
func nmNewManager() (m *nm.Manager, err error) {
	m, err = nm.NewManager(dbusNmDest, dbusNmPath)
	if err != nil {
		logger.Error(err)
		return
	}
	return
}
func nmNewDevice(devPath dbus.ObjectPath) (dev *nm.Device, err error) {
	dev, err = nm.NewDevice(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
		return
	}
	return
}
func nmNewDeviceWired(devPath dbus.ObjectPath) (dev *nm.DeviceWired, err error) {
	dev, err = nm.NewDeviceWired(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
	}
	return
}
func nmNewDeviceWireless(devPath dbus.ObjectPath) (dev *nm.DeviceWireless, err error) {
	dev, err = nm.NewDeviceWireless(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
	}
	return
}
func nmNewDeviceModem(devPath dbus.ObjectPath) (dev *nm.DeviceModem, err error) {
	dev, err = nm.NewDeviceModem(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
	}
	return
}
func nmNewDeviceBluetooth(devPath dbus.ObjectPath) (dev *nm.DeviceBluetooth, err error) {
	dev, err = nm.NewDeviceBluetooth(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
	}
	return
}
func nmNewDeviceOlpcMesh(devPath dbus.ObjectPath) (dev *nm.DeviceOlpcMesh, err error) {
	dev, err = nm.NewDeviceOlpcMesh(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
	}
	return
}
func nmNewDeviceWiMax(devPath dbus.ObjectPath) (dev *nm.DeviceWiMax, err error) {
	dev, err = nm.NewDeviceWiMax(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
	}
	return
}
func nmNewDeviceInfiniband(devPath dbus.ObjectPath) (dev *nm.DeviceInfiniband, err error) {
	dev, err = nm.NewDeviceInfiniband(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
	}
	return
}
func nmNewDeviceBond(devPath dbus.ObjectPath) (dev *nm.DeviceBond, err error) {
	dev, err = nm.NewDeviceBond(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
	}
	return
}
func nmNewDeviceBridge(devPath dbus.ObjectPath) (dev *nm.DeviceBridge, err error) {
	dev, err = nm.NewDeviceBridge(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
	}
	return
}
func nmNewDeviceVlan(devPath dbus.ObjectPath) (dev *nm.DeviceVlan, err error) {
	dev, err = nm.NewDeviceVlan(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
	}
	return
}
func nmNewDeviceAdsl(devPath dbus.ObjectPath) (dev *nm.DeviceAdsl, err error) {
	dev, err = nm.NewDeviceAdsl(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
	}
	return
}
func nmNewDeviceGeneric(devPath dbus.ObjectPath) (dev *nm.DeviceGeneric, err error) {
	dev, err = nm.NewDeviceGeneric(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
	}
	return
}
func nmNewDeviceTeam(devPath dbus.ObjectPath) (dev *nm.DeviceTeam, err error) {
	dev, err = nm.NewDeviceTeam(dbusNmDest, devPath)
	if err != nil {
		logger.Error(err)
	}
	return
}
func nmNewAccessPoint(apPath dbus.ObjectPath) (ap *nm.AccessPoint, err error) {
	ap, err = nm.NewAccessPoint(dbusNmDest, apPath)
	if err != nil {
		logger.Error(err)
		return
	}
	return
}
func nmNewActiveConnection(apath dbus.ObjectPath) (aconn *nm.ActiveConnection, err error) {
	aconn, err = nm.NewActiveConnection(dbusNmDest, apath)
	if err != nil {
		logger.Error(err)
		return
	}
	return
}
func nmNewAgentManager() (manager *nm.AgentManager, err error) {
	manager, err = nm.NewAgentManager(dbusNmDest, "/org/freedesktop/NetworkManager/AgentManager")
	if err != nil {
		logger.Error(err)
		return
	}
	return
}
func nmNewDHCP4Config(path dbus.ObjectPath) (dhcp4 *nm.DHCP4Config, err error) {
	dhcp4, err = nm.NewDHCP4Config(dbusNmDest, path)
	if err != nil {
		logger.Error(err)
		return
	}
	return
}
func nmNewDHCP6Config(path dbus.ObjectPath) (dhcp6 *nm.DHCP6Config, err error) {
	dhcp6, err = nm.NewDHCP6Config(dbusNmDest, path)
	if err != nil {
		logger.Error(err)
		return
	}
	return
}
func nmNewIP4Config(path dbus.ObjectPath) (ip4config *nm.IP4Config, err error) {
	ip4config, err = nm.NewIP4Config(dbusNmDest, path)
	if err != nil {
		logger.Error(err)
		return
	}
	return
}
func nmNewIP6Config(path dbus.ObjectPath) (ip6config *nm.IP6Config, err error) {
	ip6config, err = nm.NewIP6Config(dbusNmDest, path)
	if err != nil {
		logger.Error(err)
		return
	}
	return
}
func nmNewSettingsConnection(cpath dbus.ObjectPath) (conn *nm.SettingsConnection, err error) {
	conn, err = nm.NewSettingsConnection(dbusNmDest, cpath)
	if err != nil {
		logger.Error(err)
		return
	}
	return
}
func nmNewVpnConnection(apath dbus.ObjectPath) (vpnConn *nm.VPNConnection, err error) {
	vpnConn, err =
		nm.NewVPNConnection(dbusNmDest, apath)
	if err != nil {
		logger.Error(err)
		return
	}
	return
}

// TODO: gen code

// Destroy network manager objects
func nmDestroyManager(m *nm.Manager) {
	if m == nil {
		logger.Error("Manager to destroy is nil")
		return
	}
	nm.DestroyManager(m)
}
func nmDestroyDevice(dev *nm.Device) {
	if dev == nil {
		logger.Error("Device to destroy is nil")
		return
	}
	nm.DestroyDevice(dev)
}
func nmDestroyDeviceWired(dev *nm.DeviceWired) {
	if dev == nil {
		logger.Error("DeviceWired to destroy is nil")
		return
	}
	nm.DestroyDeviceWired(dev)
}
func nmDestroyDeviceWireless(dev *nm.DeviceWireless) {
	if dev == nil {
		logger.Error("DeviceWireless to destroy is nil")
		return
	}
	nm.DestroyDeviceWireless(dev)
}
func nmDestroyAccessPoint(ap *nm.AccessPoint) {
	if ap == nil {
		logger.Error("AccessPoint to destroy is nil")
		return
	}
	nm.DestroyAccessPoint(ap)
}
func nmDestroySettingsConnection(conn *nm.SettingsConnection) {
	if conn == nil {
		logger.Error("SettingsConnection to destroy is nil")
		return
	}
	nm.DestroySettingsConnection(conn)
}
func nmDestroyActiveConnection(aconn *nm.ActiveConnection) {
	if aconn == nil {
		logger.Error("ActiveConnection to destroy is nil")
		return
	}
	nm.DestroyActiveConnection(aconn)
}
func nmDestroyVpnConnection(vpnConn *nm.VPNConnection) {
	if vpnConn == nil {
		logger.Error("ActiveConnection to destroy is nil")
		return
	}
	nm.DestroyVPNConnection(vpnConn)
}

// Operate wrapper for network manager
func nmAgentRegister(identifier string) {
	am, err := nmNewAgentManager()
	if err != nil {
		return
	}
	err = am.Register(identifier)
	if err != nil {
		logger.Error(err)
	}
}

func nmAgentUnregister() {
	am, err := nmNewAgentManager()
	if err != nil {
		return
	}
	err = am.Unregister()
	if err != nil {
		logger.Error(err)
	}
}

func nmGetDevices() (devPaths []dbus.ObjectPath) {
	devPaths, err := nmManager.GetDevices()
	if err != nil {
		logger.Error(err)
	}
	return
}

func nmGetDevicesByType(devType uint32) (specDevPaths []dbus.ObjectPath) {
	for _, p := range nmGetDevices() {
		if dev, err := nmNewDevice(p); err == nil {
			if dev.DeviceType.Get() == devType {
				specDevPaths = append(specDevPaths, p)
			}
		}
	}
	return
}

func nmGetDeviceInterface(devPath dbus.ObjectPath) (devInterface string) {
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}
	defer nm.DestroyDevice(dev)

	devInterface = dev.Interface.Get()
	return
}

func nmGetDeviceModemCapabilities(devPath dbus.ObjectPath) (capabilities uint32) {
	devModem, err := nmNewDeviceModem(devPath)
	if err != nil {
		return
	}
	defer nm.DestroyDeviceModem(devModem)

	capabilities = devModem.CurrentCapabilities.Get()
	return
}

func nmAddAndActivateConnection(data connectionData, devPath dbus.ObjectPath) (cpath, apath dbus.ObjectPath, err error) {
	if len(devPath) == 0 {
		devPath = "/"
	}
	spath := dbus.ObjectPath("/")
	cpath, apath, err = nmManager.AddAndActivateConnection(data, devPath, spath)
	if err != nil {
		nmHandleActivatingError(data, devPath)
		logger.Error(err, "devPath:", devPath)
		return
	}
	return
}

func nmActivateConnection(cpath, devPath dbus.ObjectPath) (apath dbus.ObjectPath, err error) {
	spath := dbus.ObjectPath("/")
	apath, err = nmManager.ActivateConnection(cpath, devPath, spath)
	if err != nil {
		if data, err := nmGetConnectionData(cpath); err == nil {
			nmHandleActivatingError(data, devPath)
		}
		logger.Error(err)
		return
	}
	return
}

func nmHandleActivatingError(data connectionData, devPath dbus.ObjectPath) {
	switch nmGetDeviceType(devPath) {
	case NM_DEVICE_TYPE_ETHERNET:
		// if wired cable unplugged, give a notification
		if !isDeviceStateAvailable(nmGetDeviceState(devPath)) {
			notifyWiredCableUnplugged()
		}
	}
	switch getCustomConnectionType(data) {
	case connectionWirelessAdhoc, connectionWirelessHotspot:
		// if connection type is wireless hotspot, give a notification
		notifyApModeNotSupport()
	}
}

func nmDeactivateConnection(apath dbus.ObjectPath) (err error) {
	err = nmManager.DeactivateConnection(apath)
	if err != nil {
		logger.Error(err)
		return
	}
	return
}

func nmGetActiveConnections() (apaths []dbus.ObjectPath) {
	apaths = nmManager.ActiveConnections.Get()
	return
}

func nmGetVpnActiveConnections() (apaths []dbus.ObjectPath) {
	for _, p := range nmGetActiveConnections() {
		if aconn, err := nmNewActiveConnection(p); err == nil {
			if aconn.Vpn.Get() {
				apaths = append(apaths, p)
			}
			nm.DestroyActiveConnection(aconn)
		}
	}
	return
}

func nmGetVpnConnectionState(apath dbus.ObjectPath) (state uint32) {
	vpnConn, err := nmNewVpnConnection(apath)
	if err != nil {
		return
	}
	defer nm.DestroyVPNConnection(vpnConn)

	state = vpnConn.VpnState.Get()
	return
}

func nmGetAccessPoints(devPath dbus.ObjectPath) (apPaths []dbus.ObjectPath) {
	devWireless, err := nmNewDeviceWireless(devPath)
	if err != nil {
		return
	}
	defer nm.DestroyDeviceWireless(devWireless)

	apPaths, err = devWireless.GetAccessPoints()
	if err != nil {
		logger.Error(err)
	}
	return
}

func nmGetAccessPointSsids(devPath dbus.ObjectPath) (ssids []string) {
	for _, apPath := range nmGetAccessPoints(devPath) {
		if ap, err := nmNewAccessPoint(apPath); err == nil {
			ssids = append(ssids, string(ap.Ssid.Get()))
			nm.DestroyAccessPoint(ap)
		}
	}
	return
}

func nmGetManagerState() (state uint32) {
	state = nmManager.State.Get()
	return
}

func nmGetActiveConnectionByUuid(uuid string) (apaths []dbus.ObjectPath, err error) {
	for _, apath := range nmGetActiveConnections() {
		if aconn, tmperr := nmNewActiveConnection(apath); tmperr == nil {
			defer nm.DestroyActiveConnection(aconn)
			if aconn.Uuid.Get() == uuid {
				apaths = append(apaths, apath)
				return
			}
		}
	}
	err = fmt.Errorf("not found active connection with uuid %s", uuid)
	return
}

func nmGetActiveConnectionState(apath dbus.ObjectPath) (state uint32) {
	aconn, err := nmNewActiveConnection(apath)
	if err != nil {
		return
	}
	defer nm.DestroyActiveConnection(aconn)

	state = aconn.State.Get()
	return
}

func nmGetActiveConnectionVpn(apath dbus.ObjectPath) (isVpn bool) {
	aconn, err := nmNewActiveConnection(apath)
	if err != nil {
		return
	}
	defer nm.DestroyActiveConnection(aconn)

	isVpn = aconn.Vpn.Get()
	return
}

func nmGetConnectionData(cpath dbus.ObjectPath) (data connectionData, err error) {
	nmConn, err := nmNewSettingsConnection(cpath)
	if err != nil {
		return
	}
	defer nm.DestroySettingsConnection(nmConn)

	data, err = nmConn.GetSettings()
	if err != nil {
		logger.Error(err)
		return
	}
	return
}

func nmUpdateConnectionData(cpath dbus.ObjectPath, data connectionData) (err error) {
	nmConn, err := nmNewSettingsConnection(cpath)
	if err != nil {
		return
	}
	defer nm.DestroySettingsConnection(nmConn)

	err = nmConn.Update(data)
	if err != nil {
		logger.Error(err)
	}
	return
}

func nmGetConnectionSecrets(cpath dbus.ObjectPath, secretField string) (secrets connectionData, err error) {
	nmConn, err := nmNewSettingsConnection(cpath)
	if err != nil {
		return
	}
	defer nm.DestroySettingsConnection(nmConn)

	secrets, err = nmConn.GetSecrets(secretField)
	if err != nil {
		logger.Error(err)
		return
	}
	return
}

func nmSetConnectionAutoconnect(cpath dbus.ObjectPath, autoConnect bool) (err error) {
	data, err := nmGetConnectionData(cpath)
	if err != nil {
		return
	}
	setSettingConnectionAutoconnect(data, autoConnect)
	return nmUpdateConnectionData(cpath, data)
}
func nmGetConnectionAutoconnect(cpath dbus.ObjectPath) (autoConnect bool) {
	data, err := nmGetConnectionData(cpath)
	if err != nil {
		return
	}
	autoConnect = getSettingConnectionAutoconnect(data)
	return
}

func nmGetConnectionId(cpath dbus.ObjectPath) (id string) {
	data, err := nmGetConnectionData(cpath)
	if err != nil {
		return
	}
	id = getSettingConnectionId(data)
	if len(id) == 0 {
		logger.Error("get Id of connection failed, id is empty")
	}
	return
}
func nmSetConnectionId(cpath dbus.ObjectPath, id string) (err error) {
	data, err := nmGetConnectionData(cpath)
	if err != nil {
		return
	}
	setSettingConnectionId(data, id)
	return nmUpdateConnectionData(cpath, data)
}

func nmGetConnectionUuid(cpath dbus.ObjectPath) (uuid string, err error) {
	data, err := nmGetConnectionData(cpath)
	if err != nil {
		return
	}
	uuid = getSettingConnectionUuid(data)
	return
}

func nmGetConnectionType(cpath dbus.ObjectPath) (ctype string) {
	data, err := nmGetConnectionData(cpath)
	if err != nil {
		return
	}
	ctype = getSettingConnectionType(data)
	if len(ctype) == 0 {
		logger.Error("get type of connection failed, type is empty")
	}
	return
}

func nmGetConnectionList() (connections []dbus.ObjectPath) {
	connections, err := nmSettings.ListConnections()
	if err != nil {
		logger.Error(err)
		return
	}
	return
}

func nmGetConnectionUuids() (uuids []string) {
	for _, cpath := range nmGetConnectionList() {
		if uuid, err := nmGetConnectionUuid(cpath); err == nil {
			uuids = append(uuids, uuid)
		}
	}
	return
}

func nmGetConnectionUuidsByType(connTypes ...string) (uuids []string) {
	for _, cpath := range nmGetConnectionList() {
		if isStringInArray(nmGetConnectionType(cpath), connTypes) {
			if uuid, err := nmGetConnectionUuid(cpath); err == nil {
				uuids = append(uuids, uuid)
			}
		}
	}
	return
}

func nmGetConnectionIds() (ids []string) {
	for _, cpath := range nmGetConnectionList() {
		ids = append(ids, nmGetConnectionId(cpath))
	}
	return
}

func nmGetConnectionById(id string) (cpath dbus.ObjectPath, err error) {
	for _, cpath = range nmGetConnectionList() {
		data, tmperr := nmGetConnectionData(cpath)
		if tmperr != nil {
			continue
		}
		if getSettingConnectionId(data) == id {
			return
		}
	}
	err = fmt.Errorf("connection with id %s not found", id)
	return
}

func nmGetConnectionByUuid(uuid string) (cpath dbus.ObjectPath, err error) {
	cpath, err = nmSettings.GetConnectionByUuid(uuid)
	return
}

// get wireless connection by ssid, the connection with special hardware address is priority
// TODO: use available connections instead
func nmGetWirelessConnection(ssid []byte, devPath dbus.ObjectPath) (cpath dbus.ObjectPath, ok bool) {
	var hwAddr string
	if len(devPath) != 0 {
		hwAddr, _ = nmGeneralGetDeviceHwAddr(devPath)
	}
	ok = false
	for _, p := range nmGetWirelessConnectionListBySsid(ssid) {
		data, err := nmGetConnectionData(p)
		if err != nil {
			continue
		}
		if isSettingWirelessMacAddressExists(data) {
			if hwAddr == convertMacAddressToString(getSettingWirelessMacAddress(data)) {
				cpath = p
				ok = true
				return
			}
		} else if !ok {
			cpath = p
			ok = true
		}
	}
	return
}

func nmGetWirelessConnectionListBySsid(ssid []byte) (cpaths []dbus.ObjectPath) {
	for _, p := range nmGetConnectionList() {
		data, err := nmGetConnectionData(p)
		if err != nil {
			continue
		}
		if getCustomConnectionType(data) != connectionWireless {
			continue
		}
		if isSettingWirelessSsidExists(data) && string(getSettingWirelessSsid(data)) == string(ssid) {
			cpaths = append(cpaths, p)
		}
	}
	return
}

func nmGetConnectionSsidByUuid(uuid string) (ssid []byte) {
	cpath, err := nmGetConnectionByUuid(uuid)
	if err != nil {
		return
	}
	data, err := nmGetConnectionData(cpath)
	if err != nil {
		return
	}
	ssid = getSettingWirelessSsid(data)
	return
}

func nmAddConnection(data connectionData) (cpath dbus.ObjectPath, err error) {
	cpath, err = nmSettings.AddConnection(data)
	if err != nil {
		logger.Error(err)
	}
	return
}

// TODO: remove, use nmGetIp4ConfigInfo instead
func nmGetDhcp4Info(path dbus.ObjectPath) (ip, mask string, routers, nameServers []string) {
	ip = "0.0.0.0"
	mask = "0.0.0.0"
	routers = make([]string, 0)
	nameServers = make([]string, 0)

	dhcp4, err := nmNewDHCP4Config(path)
	if err != nil {
		return
	}
	defer nm.DestroyDHCP4Config(dhcp4)

	options := dhcp4.Options.Get()
	if ipData, ok := options["ip_address"]; ok {
		ip, _ = ipData.Value().(string)
	}
	if maskData, ok := options["subnet_mask"]; ok {
		mask, _ = maskData.Value().(string)
	}
	if routersData, ok := options["routers"]; ok {
		routersStr, _ := routersData.Value().(string)
		if len(routersStr) > 0 {
			routers = strings.Split(routersStr, " ")
		}
	}
	if nameServersData, ok := options["domain_name_servers"]; ok {
		nameServersStr, _ := nameServersData.Value().(string)
		if len(nameServersStr) > 0 {
			nameServers = strings.Split(nameServersStr, " ")
		}
	}
	return
}

// TODO: remove, use nmGetIp6ConfigInfo instead
func nmGetDhcp6Info(path dbus.ObjectPath) (ip string, routers, nameServers []string) {
	ip = "0::0"
	routers = make([]string, 0)
	nameServers = make([]string, 0)

	dhcp6, err := nmNewDHCP6Config(path)
	if err != nil {
		return
	}
	defer nm.DestroyDHCP6Config(dhcp6)

	options := dhcp6.Options.Get()
	if ipData, ok := options["ip6_address"]; ok {
		ip, _ = ipData.Value().(string)
	}
	if routersData, ok := options["routers"]; ok {
		routersStr, _ := routersData.Value().(string)
		if len(routersStr) > 0 {
			routers = strings.Split(routersStr, " ")
		}
	}
	if nameServersData, ok := options["dhcp6_name_servers"]; ok {
		nameServersStr, _ := nameServersData.Value().(string)
		if len(nameServersStr) > 0 {
			nameServers = strings.Split(nameServersStr, " ")
		}
	}
	return
}

func nmGetIp4ConfigInfo(path dbus.ObjectPath) (address, mask string, gateways, nameServers []string) {
	address = "0.0.0.0"
	mask = "0.0.0.0"
	ip4config, err := nmNewIP4Config(path)
	if err != nil {
		return
	}
	defer nm.DestroyIP4Config(ip4config)

	ipv4Addresses := wrapIpv4Addresses(ip4config.Addresses.Get())
	if len(ipv4Addresses) > 0 {
		address = ipv4Addresses[0].Address
		mask = ipv4Addresses[0].Mask
	}
	for _, address := range ipv4Addresses {
		gateways = append(gateways, address.Gateway)
	}

	nameServers = wrapIpv4Dns(ip4config.Nameservers.Get())
	return
}

func nmGetIp6ConfigInfo(path dbus.ObjectPath) (address, prefix string, gateways, nameServers []string) {
	address = "0::0"
	prefix = "0"
	ip6config, err := nmNewIP6Config(path)
	if err != nil {
		return
	}
	defer nm.DestroyIP6Config(ip6config)

	ipv6Addresses := wrapIpv6Addresses(interfaceToIpv6Addresses(ip6config.Addresses.Get()))
	if len(ipv6Addresses) > 0 {
		address = ipv6Addresses[0].Address
		prefix = fmt.Sprintf("%d", ipv6Addresses[0].Prefix)
	}
	for _, address := range ipv6Addresses {
		gateways = append(gateways, address.Gateway)
	}

	nameServers = wrapIpv6Dns(ip6config.Nameservers.Get())
	return
}

func nmGetDeviceState(devPath dbus.ObjectPath) (state uint32) {
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return NM_DEVICE_STATE_UNKNOWN
	}
	defer nm.DestroyDevice(dev)

	state = dev.State.Get()
	return
}

func nmGetDeviceAutoconnect(devPath dbus.ObjectPath) (autoconnect bool) {
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}
	defer nm.DestroyDevice(dev)

	autoconnect = dev.Autoconnect.Get()
	return
}
func nmSetDeviceAutoconnect(devPath dbus.ObjectPath, autoconnect bool) {
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}
	defer nm.DestroyDevice(dev)

	dev.Autoconnect.Set(autoconnect)
	return
}

func nmGetDeviceType(devPath dbus.ObjectPath) (devType uint32) {
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return NM_DEVICE_TYPE_UNKNOWN
	}
	defer nm.DestroyDevice(dev)

	devType = dev.DeviceType.Get()
	return
}

func nmGetDeviceUdi(devPath dbus.ObjectPath) (udi string) {
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}
	defer nm.DestroyDevice(dev)

	udi = dev.Udi.Get()
	return
}

func nmGetDeviceActiveConnection(devPath dbus.ObjectPath) (acPath dbus.ObjectPath) {
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}
	defer nm.DestroyDevice(dev)

	acPath = dev.ActiveConnection.Get()
	return
}

func nmGetDeviceAvailableConnections(devPath dbus.ObjectPath) (paths []dbus.ObjectPath) {
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}
	defer nm.DestroyDevice(dev)

	paths = dev.AvailableConnections.Get()
	return
}

func nmGetDeviceActiveConnectionUuid(devPath dbus.ObjectPath) (uuid string, err error) {
	acPath := nmGetDeviceActiveConnection(devPath)
	aconn, err := nmNewActiveConnection(acPath)
	if err != nil {
		return
	}
	defer nm.DestroyActiveConnection(aconn)

	uuid = aconn.Uuid.Get()
	return
}

func nmGetDeviceActiveConnectionData(devPath dbus.ObjectPath) (data connectionData, err error) {
	if !isDeviceStateInActivating(nmGetDeviceState(devPath)) {
		err = fmt.Errorf("device is inactivated %s", devPath)
		return
	}
	acPath := nmGetDeviceActiveConnection(devPath)
	aconn, err := nmNewActiveConnection(acPath)
	if err != nil {
		return
	}
	defer nm.DestroyActiveConnection(aconn)

	conn, err := nmNewSettingsConnection(aconn.Connection.Get())
	if err != nil {
		return
	}
	defer nm.DestroySettingsConnection(conn)

	data, err = conn.GetSettings()
	if err != nil {
		logger.Error(err)
		return
	}
	return
}

func nmManagerEnable(enable bool) (err error) {
	err = nmManager.Enable(enable)
	if err != nil {
		logger.Error(err)
	}
	return
}

func nmGetPrimaryConnection() (cpath dbus.ObjectPath) {
	// TODO need update dbus-factory
	// cpath = nmManager.PrimaryConnection.Get()
	cpath = ""
	return
}

func nmGetNetworkState() uint32 {
	return nmManager.State.Get()
}
func nmIsNetworkOffline() bool {
	state := nmManager.State.Get()
	if state == NM_STATE_DISCONNECTED || state == NM_STATE_ASLEEP {
		return true
	}
	return false
}

func nmGetNetworkEnabled() bool {
	return nmManager.NetworkingEnabled.Get()
}
func nmGetWirelessHardwareEnabled() bool {
	return nmManager.WirelessHardwareEnabled.Get()
}
func nmGetWirelessEnabled() bool {
	return nmManager.WirelessEnabled.Get()
}

func nmSetNetworkingEnabled(enabled bool) {
	if nmManager.NetworkingEnabled.Get() != enabled {
		nmManagerEnable(enabled)
	} else {
		logger.Warning("NetworkingEnabled already set as", enabled)
	}
	return
}
func nmSetWirelessEnabled(enabled bool) {
	if nmManager.WirelessEnabled.Get() != enabled {
		nmManager.WirelessEnabled.Set(enabled)
	} else {
		logger.Warning("WirelessEnabled already set as", enabled)
	}
	return
}
func nmSetWwanEnabled(enabled bool) {
	if nmManager.WwanEnabled.Get() != enabled {
		nmManager.WwanEnabled.Set(enabled)
	} else {
		logger.Warning("WwanEnabled already set as", enabled)
	}
}

type autoConnectConn struct {
	id        string
	uuid      string
	timestamp uint64
}
type autoConnectConns []autoConnectConn

func (acs autoConnectConns) Len() int {
	return len(acs)
}
func (acs autoConnectConns) Swap(i, j int) {
	acs[i], acs[j] = acs[j], acs[i]
}
func (acs autoConnectConns) Less(i, j int) bool {
	return acs[i].timestamp < acs[j].timestamp
}
func nmGetConnectionUuidsForAutoConnect(devPath dbus.ObjectPath, lastConnectionUuid string) (uuids []string) {
	acs := make(autoConnectConns, 0)
	devRelatedUuid := nmGeneralGetDeviceUniqueUuid(devPath)
	for _, cpath := range nmGetDeviceAvailableConnections(devPath) {
		if cdata, err := nmGetConnectionData(cpath); err == nil {
			uuid := getSettingConnectionUuid(cdata)
			switch getCustomConnectionType(cdata) {
			case connectionWired, connectionMobileGsm, connectionMobileCdma:
				if devRelatedUuid != uuid {
					// ignore connections that not matching the
					// device, etc wired connections that create in
					// other ways
					continue
				}
			}
			if uuid == lastConnectionUuid {
				// the last activated connection will be dispatch
				// specially
				continue
			}
			if getSettingConnectionAutoconnect(cdata) {
				id := getSettingConnectionId(cdata)
				timestamp := getSettingConnectionTimestamp(cdata)
				if timestamp > 0 {
					// only collect connections that connected before
					ac := autoConnectConn{
						id:        id,
						uuid:      uuid,
						timestamp: timestamp,
					}
					acs = append(acs, ac)
				}
			}
		}
	}
	sort.Sort(sort.Reverse(acs))
	logger.Debugf("autoconnect connections for device type %s, %v",
		getCustomDeviceType(nmGetDeviceType(devPath)), acs)
	if len(lastConnectionUuid) > 0 {
		// the last activated connection has the highest priority if
		// exists and the auto-connect property enabled
		if cpath, err := nmGetConnectionByUuid(lastConnectionUuid); err == nil {
			if nmGetConnectionAutoconnect(cpath) {
				uuids = []string{lastConnectionUuid}
			}
		}
	}
	for _, ac := range acs {
		uuids = append(uuids, ac.uuid)
	}
	return
}

func nmRunOnceUntilDeviceAvailable(devPath dbus.ObjectPath, cb func()) {
	dev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}
	defer nm.DestroyDevice(dev)

	state := dev.State.Get()
	if isDeviceStateAvailable(state) {
		cb()
	} else {
		hasRun := false
		dev.ConnectStateChanged(func(newState uint32, oldState uint32, reason uint32) {
			if !hasRun && isDeviceStateAvailable(newState) {
				cb()
				nmDestroyDevice(dev)
				hasRun = true
			}
		})
	}
}

func nmRunOnceUtilNetworkAvailable(cb func()) {
	nm, err := nmNewManager()
	if err != nil {
		return
	}
	state := nm.State.Get()
	if state >= NM_STATE_CONNECTED_LOCAL {
		cb()
	} else {
		hasRun := false
		nm.ConnectStateChanged(func(state uint32) {
			if !hasRun && state >= NM_STATE_CONNECTED_LOCAL {
				cb()
				nmDestroyManager(nm)
				hasRun = true
			}
		})
	}
}
