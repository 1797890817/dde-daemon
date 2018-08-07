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

package power

import (
	"sync"

	"gir/gio-2.0"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/dbusutil/gsprop"
	"pkg.deepin.io/lib/dbusutil/proxy"
)

//go:generate dbusutil-gen -type Manager manager.go
type Manager struct {
	service              *dbusutil.Service
	sysSigLoop           *dbusutil.SignalLoop
	sysDBusDaemon        *ofdbus.DBus
	helper               *Helper
	settings             *gio.Settings
	isSuspending         bool
	warnLevelCountTicker *countTicker
	warnLevelConfig      *WarnLevelConfigManager
	submodules           map[string]submodule
	inhibitor            *sleepInhibitor

	PropsMu sync.RWMutex
	// 是否有盖子，一般笔记本电脑才有
	LidIsPresent bool
	// 是否使用电池, 接通电源时为 false, 使用电池时为 true
	OnBattery bool

	// 警告级别
	WarnLevel WarnLevel

	// 是否有环境光传感器
	HasAmbientLightSensor bool

	// dbusutil-gen: ignore-below
	// 电池是否可用，是否存在
	BatteryIsPresent map[string]bool
	// 电池电量百分比
	BatteryPercentage map[string]float64
	// 电池状态
	BatteryState map[string]uint32

	// 接通电源时，不做任何操作，到关闭屏幕需要的时间
	LinePowerScreenBlackDelay gsprop.Int `prop:"access:rw"`
	// 接通电源时，不做任何操作，从黑屏到睡眠的时间
	LinePowerSleepDelay gsprop.Int `prop:"access:rw"`

	// 使用电池时，不做任何操作，到关闭屏幕需要的时间
	BatteryScreenBlackDelay gsprop.Int `prop:"access:rw"`
	// 使用电池时，不做任何操作，从黑屏到睡眠的时间
	BatterySleepDelay gsprop.Int `prop:"access:rw"`

	// 关闭显示器前是否锁定
	ScreenBlackLock gsprop.Bool `prop:"access:rw"`
	// 睡眠前是否锁定
	SleepLock gsprop.Bool `prop:"access:rw"`

	// 笔记本电脑盖上盖子后是否睡眠
	LidClosedSleep gsprop.Bool `prop:"access:rw"`

	AmbientLightAdjustBrightness gsprop.Bool `prop:"access:rw"`
	ambientLightClaimed          bool
	lightLevelUnit               string
	lidSwitchState               uint
	sessionActive                bool
}

func NewManager(service *dbusutil.Service) (*Manager, error) {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	helper, err := NewHelper()
	if err != nil {
		return nil, err
	}
	m := new(Manager)
	m.service = service
	m.helper = helper

	m.settings = gio.NewSettings(gsSchemaPower)
	m.warnLevelConfig = NewWarnLevelConfigManager(m.settings)

	m.LinePowerScreenBlackDelay.Bind(m.settings, settingKeyLinePowerScreenBlackDelay)
	m.LinePowerSleepDelay.Bind(m.settings, settingKeyLinePowerSleepDelay)
	m.BatteryScreenBlackDelay.Bind(m.settings, settingKeyBatteryScreenBlackDelay)
	m.BatterySleepDelay.Bind(m.settings, settingKeyBatterySleepDelay)
	m.ScreenBlackLock.Bind(m.settings, settingKeyScreenBlackLock)
	m.SleepLock.Bind(m.settings, settingKeySleepLock)
	m.LidClosedSleep.Bind(m.settings, settingKeyLidClosedSleep)
	m.AmbientLightAdjustBrightness.Bind(m.settings,
		settingKeyAmbientLightAdjuestBrightness)

	power := m.helper.Power
	m.LidIsPresent = power.HasLidSwitch.Get()
	m.OnBattery = power.OnBattery.Get()
	logger.Info("LidIsPresent", m.LidIsPresent)
	m.HasAmbientLightSensor = helper.SensorProxy.HasAmbientLight.Get()
	logger.Debug("HasAmbientLightSensor:", m.HasAmbientLightSensor)
	if m.HasAmbientLightSensor {
		m.lightLevelUnit = helper.SensorProxy.LightLevelUnit.Get()
	}

	m.sessionActive = helper.SessionWatcher.IsActive.Get()

	// init battery display
	m.BatteryIsPresent = make(map[string]bool)
	m.BatteryPercentage = make(map[string]float64)
	m.BatteryState = make(map[string]uint32)

	m.sysSigLoop = dbusutil.NewSignalLoop(systemBus, 10)
	m.sysDBusDaemon = ofdbus.NewDBus(systemBus)

	m.claimOrReleaseAmbientLight()
	return m, nil
}

func (m *Manager) init() {
	// init sleep inhibitor
	m.inhibitor = newSleepInhibitor(m.helper.Login1Manager)
	m.inhibitor.OnBeforeSuspend = m.handleBeforeSuspend
	m.inhibitor.OnWakeup = m.handleWakeup
	m.inhibitor.block()

	m.handleBatteryDisplayUpdate()
	power := m.helper.Power
	power.ConnectBatteryDisplayUpdate(func(timestamp int64) {
		logger.Debug("BatteryDisplayUpdate", timestamp)
		m.handleBatteryDisplayUpdate()
	})

	m.helper.SensorProxy.LightLevel.ConnectChanged(func() {
		m.handleLightLevelChanged()
	})

	m.helper.SessionWatcher.IsActive.ConnectChanged(func() {
		isActive := m.helper.SessionWatcher.IsActive.Get()
		m.sessionActive = isActive
		logger.Debug("session active changed to:", isActive)
		m.claimOrReleaseAmbientLight()
	})

	m.sysSigLoop.Start()
	m.sysDBusDaemon.InitSignalExt(m.sysSigLoop, true)
	m.sysDBusDaemon.ConnectNameOwnerChanged(
		func(name string, oldOwner string, newOwner string) {
			if name == "net.hadess.SensorProxy" && newOwner != "" {
				logger.Debug("sensorProxy restarted")
				hasSensor := m.helper.SensorProxy.HasAmbientLight.Get()
				m.setPropHasAmbientLightSensor(hasSensor)
				m.ambientLightClaimed = false
				m.claimOrReleaseAmbientLight()
			}
		})

	m.warnLevelConfig.setChangeCallback(m.handleBatteryDisplayUpdate)

	m.initPowerModule()

	m.initOnBatteryChangedHandler()
	m.initSubmodules()
	m.startSubmodules()
}

func (m *Manager) initPowerModule() {
	init := m.settings.GetBoolean(settingKeyPowerModuleInitialized)
	if !init {
		// TODO: 也许有更好的判断台式机的方法
		power := m.helper.Power
		if !power.HasBattery.Get() {
			// 无电池，判断为台式机, 设置待机为 从不
			m.LinePowerSleepDelay.Set(0)
			m.BatterySleepDelay.Set(0)
		}
		m.settings.SetBoolean(settingKeyPowerModuleInitialized, true)
	}
}

func (m *Manager) isX11SessionActive() (bool, error) {
	return m.helper.SessionWatcher.IsX11SessionActive()
}

func (m *Manager) destroy() {
	m.destroySubmodules()
	m.releaseAmbientLight()

	if m.helper != nil {
		m.helper.Destroy()
		m.helper = nil
	}

	if m.inhibitor != nil {
		m.inhibitor.unblock()
		m.inhibitor = nil
	}

	m.sysDBusDaemon.RemoveHandler(proxy.RemoveAllHandlers)
	m.sysSigLoop.Stop()

}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) Reset() *dbus.Error {
	logger.Debug("Reset settings")

	var settingKeys = []string{
		settingKeyLinePowerScreenBlackDelay,
		settingKeyLinePowerSleepDelay,
		settingKeyBatteryScreenBlackDelay,
		settingKeyBatterySleepDelay,
		settingKeyScreenBlackLock,
		settingKeySleepLock,
		settingKeyLidClosedSleep,
		settingKeyPowerButtonPressedExec,
	}
	for _, key := range settingKeys {
		logger.Debug("reset setting", key)
		m.settings.Reset(key)
	}
	return nil
}
