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
	"fmt"
	"pkg.linuxdeepin.com/lib/dbus"
	"pkg.linuxdeepin.com/lib/utils"
)

// config structure wrapper
type config struct {
	core utils.Config

	WiredEnabled        bool
	VpnEnabled          bool
	LastWirelessEnabled bool
	LastWwanEnabled     bool
	LastWiredEnabled    bool
	LastVpnEnabled      bool

	Devices        map[string]*deviceConfig // config for each device
	VpnConnections map[string]*vpnConfig    // config for each vpn connection
}

type deviceConfig struct {
	Enabled            bool
	LastEnabled        bool
	LastConnectionUuid string
}

type vpnConfig struct {
	AutoConnect bool

	// don't need to save activated state
	activated     bool
	lastActivated bool
}

func newConfig() (c *config) {
	c = &config{}
	c.core.SetConfigName("network")
	logger.Info("config file:", c.core.GetConfigFile())
	c.Devices = make(map[string]*deviceConfig)
	c.VpnConnections = make(map[string]*vpnConfig)
	c.WiredEnabled = true
	c.VpnEnabled = false
	c.LastWirelessEnabled = true
	c.LastWwanEnabled = true
	c.LastWiredEnabled = c.WiredEnabled
	c.LastVpnEnabled = c.VpnEnabled
	c.load()
	c.clearSpareConfig()
	return
}
func (c *config) save() {
	c.core.Save(c)
}
func (c *config) load() {
	c.core.Load(c)
}

func newDeviceConfig() (d *deviceConfig) {
	d = &deviceConfig{}
	d.Enabled = true
	d.LastEnabled = d.Enabled
	return
}

func newVpnConfig() (v *vpnConfig) {
	v = &vpnConfig{}
	v.AutoConnect = true
	v.activated = false
	v.lastActivated = v.activated
	return
}

func (c *config) clearSpareConfig() {
	// remove spare device and vpn config
	devIds := nmGetDeviceIdentifiers()
	for id, _ := range c.Devices {
		if !isStringInArray(id, devIds) {
			c.removeDeviceConfig(id)
		}
	}
	vpnUuids := nmGetConnectionUuidsByType(NM_SETTING_VPN_SETTING_NAME)
	for uuid, _ := range c.VpnConnections {
		if !isStringInArray(uuid, vpnUuids) {
			c.removeVpnConfig(uuid)
		}
	}
}

func (c *config) getLastGlobalSwithes() bool {
	return c.LastWirelessEnabled
}
func (c *config) getLastWirelessEnabled() bool {
	return c.LastWirelessEnabled
}
func (c *config) getLastWwanEnabled() bool {
	return c.LastWwanEnabled
}
func (c *config) getLastWiredEnabled() bool {
	return c.LastWiredEnabled
}
func (c *config) getLastVpnEnabled() bool {
	return c.LastVpnEnabled
}

func (c *config) setLastGlobalSwithes(enabled bool) {
	c.LastWirelessEnabled = enabled
	c.LastWwanEnabled = enabled
	c.LastWiredEnabled = enabled
	c.LastVpnEnabled = enabled
	c.save()
}
func (c *config) setLastWirelessEnabled(enabled bool) {
	if c.LastWirelessEnabled != enabled {
		c.LastWirelessEnabled = enabled
		c.save()
	}
}
func (c *config) setLastWwanEnabled(enabled bool) {
	if c.LastWwanEnabled != enabled {
		c.LastWwanEnabled = enabled
		c.save()
	}
}
func (c *config) setLastWiredEnabled(enabled bool) {
	if c.LastWiredEnabled != enabled {
		c.LastWiredEnabled = enabled
		c.save()
	}
}
func (c *config) setLastVpnEnabled(enabled bool) {
	if c.LastVpnEnabled != enabled {
		c.LastVpnEnabled = enabled
		c.save()
	}
}

func (c *config) getWiredEnabled() bool {
	return c.WiredEnabled
}
func (c *config) getVpnEnabled() bool {
	return c.VpnEnabled
}

func (c *config) setWiredEnabled(enabled bool) {
	if c.WiredEnabled != enabled {
		c.WiredEnabled = enabled
		c.save()
	}
}
func (c *config) setVpnEnabled(enabled bool) {
	if c.VpnEnabled != enabled {
		c.VpnEnabled = enabled
		c.save()
	}
}

func (c *config) removeConnection(uuid string) {
	for _, devConfig := range c.Devices {
		if devConfig.LastConnectionUuid == uuid {
			devConfig.LastConnectionUuid = ""
		}
	}
	c.removeVpnConfig(uuid)
	c.save()
}

// deviceConfig related functions
func (c *config) isDeviceConfigExists(devId string) (ok bool) {
	_, ok = c.Devices[devId]
	return
}
func (c *config) getDeviceConfigForPath(devPath dbus.ObjectPath) (d *deviceConfig, err error) {
	devId, err := nmGeneralGetDeviceIdentifier(devPath)
	if err != nil {
		return
	}
	return c.getDeviceConfig(devId)
}
func (c *config) getDeviceConfig(devId string) (d *deviceConfig, err error) {
	if !c.isDeviceConfigExists(devId) {
		err = fmt.Errorf("device config for %s not exists", devId)
		logger.Warning(err)
		return
	}
	d, _ = c.Devices[devId]
	return
}
func (c *config) addDeviceConfig(devPath dbus.ObjectPath) {
	devId, err := nmGeneralGetDeviceIdentifier(devPath)
	if err != nil {
		return
	}
	if !c.isDeviceConfigExists(devId) {
		devConfig := newDeviceConfig()
		devConfig.LastConnectionUuid, _ = nmGetDeviceActiveConnectionUuid(devPath)
		c.Devices[devId] = devConfig
		c.save()
	}
}
func (c *config) removeDeviceConfig(devId string) {
	if !c.isDeviceConfigExists(devId) {
		logger.Errorf("device config for %s not exists", devId)
	}
	delete(c.Devices, devId)
	c.save()
}
func (c *config) updateDeviceConfig(devPath dbus.ObjectPath) {
	devConfig, err := c.getDeviceConfigForPath(devPath)
	if err != nil {
		return
	}
	devState := nmGetDeviceState(devPath)
	if devConfig.Enabled {
		if isDeviceStateInActivating(devState) {
			devConfig.LastConnectionUuid, _ = nmGetDeviceActiveConnectionUuid(devPath)
			c.save()
		}
	}
}
func (c *config) syncDeviceState(devPath dbus.ObjectPath) {
	devConfig, err := c.getDeviceConfigForPath(devPath)
	if err != nil {
		return
	}
	devState := nmGetDeviceState(devPath)
	if isDeviceStateInActivating(devState) {
		// sync device state
		if !devConfig.Enabled {
			manager.doDisconnectDevice(devPath)
		}
	}
}
func (c *config) getDeviceEnabled(devPath dbus.ObjectPath) (enabled bool) {
	devConfig, err := c.getDeviceConfigForPath(devPath)
	if err != nil {
		enabled = true // return true as default
		return
	}
	enabled = devConfig.Enabled
	return
}
func (c *config) setDeviceEnabled(devPath dbus.ObjectPath, enabled bool) {
	devConfig, err := c.getDeviceConfigForPath(devPath)
	if err != nil {
		return
	}
	devConfig.Enabled = enabled
	dbus.Emit(manager, "DeviceEnabled", string(devPath), enabled)
	c.save()
}
func (c *config) setDeviceLastConnectionUuid(devPath dbus.ObjectPath, uuid string) {
	devConfig, err := c.getDeviceConfigForPath(devPath)
	if err != nil {
		return
	}
	if devConfig.LastConnectionUuid != uuid {
		devConfig.LastConnectionUuid = uuid
		c.save()
	}
}
func (c *config) setDeviceLastEnabled(devPath dbus.ObjectPath, enabled bool) {
	devConfig, err := c.getDeviceConfigForPath(devPath)
	if err != nil {
		return
	}
	if devConfig.LastEnabled != enabled {
		devConfig.LastEnabled = enabled
		c.save()
	}
}
func (c *config) setAllDeviceLastEnabled(enabled bool) {
	for _, devConfig := range c.Devices {
		devConfig.LastEnabled = enabled
	}
	c.save()
}

func (c *config) saveDeviceState(devPath dbus.ObjectPath) {
	devConfig, err := c.getDeviceConfigForPath(devPath)
	if err != nil {
		return
	}
	if devConfig.LastEnabled != devConfig.Enabled {
		devConfig.LastEnabled = devConfig.Enabled
		c.save()
	}
}
func (c *config) restoreDeviceState(devPath dbus.ObjectPath) {
	devConfig, err := c.getDeviceConfigForPath(devPath)
	if err != nil {
		return
	}
	if devConfig.Enabled != devConfig.LastEnabled {
		devConfig.Enabled = devConfig.LastEnabled
		c.save()
	}
}

// vpnConfig related functions
func (c *config) isVpnConfigExists(uuid string) (ok bool) {
	_, ok = c.VpnConnections[uuid]
	return
}
func (c *config) getVpnConfig(uuid string) (v *vpnConfig, err error) {
	if !c.isVpnConfigExists(uuid) {
		err = fmt.Errorf("vpn config for %s not exists", uuid)
		logger.Warning(err)
		return
	}
	v, _ = c.VpnConnections[uuid]
	return
}
func (c *config) addVpnConfig(uuid string) {
	if !c.isVpnConfigExists(uuid) {
		vpnConfig := newVpnConfig()
		c.VpnConnections[uuid] = vpnConfig
		c.save()
	}
}
func (c *config) removeVpnConfig(uuid string) {
	if c.isVpnConfigExists(uuid) {
		delete(c.VpnConnections, uuid)
		c.save()
	}
}
func (c *config) setVpnConnectionActivated(uuid string, activated bool) {
	vpnConfig, err := c.getVpnConfig(uuid)
	if err != nil {
		return
	}
	if vpnConfig.activated != activated {
		vpnConfig.activated = activated
		c.save()
	}
}
func (c *config) isVpnConnectionAutoConnect(uuid string) bool {
	vpnConfig, err := c.getVpnConfig(uuid)
	if err != nil {
		return false
	}
	return vpnConfig.AutoConnect
}
func (c *config) setVpnConnectionAutoConnect(uuid string, autoConnect bool) {
	vpnConfig, err := c.getVpnConfig(uuid)
	if err != nil {
		return
	}
	if vpnConfig.AutoConnect != autoConnect {
		vpnConfig.AutoConnect = autoConnect
		c.save()
	}
}
