/*
 * Copyright (C) 2014 ~ 2017 Deepin Technology Co., Ltd.
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

package inputdevices

import (
	"sync"

	"gir/gio-2.0"
	"pkg.deepin.io/dde/api/dxinput"
)

var gsLocker sync.Mutex

func (m *Manager) handleGSettings() {
	m.settings.Connect("changed", func(s *gio.Settings, key string) {
		switch key {
		case gsKeyWheelSpeed:
			m.setWheelSpeed(false)
		}
	})
}

func (kbd *Keyboard) handleGSettings() {
	kbd.setting.Connect("changed", func(s *gio.Settings, key string) {
		switch key {
		case kbdKeyRepeatEnable, kbdKeyRepeatDelay,
			kbdKeyRepeatInterval:
			kbd.setRepeat()
		case kbdKeyLayout:
			kbd.setLayout()
		case kbdKeyCursorBlink:
			kbd.setCursorBlink()
		case kbdKeyLayoutOptions:
			kbd.setOptions()
		case kbdKeyUserLayoutList:
			kbd.setGreeterLayoutList()
		}
	})
}

func (m *Mouse) handleGSettings() {
	m.setting.Connect("changed", func(s *gio.Settings, key string) {
		switch key {
		case mouseKeyLeftHanded:
			m.enableLeftHanded()
		case mouseKeyDisableTouchpad:
			m.disableTouchpad()
		case mouseKeyNaturalScroll:
			m.enableNaturalScroll()
		case mouseKeyMiddleButton:
			m.enableMidBtnEmu()
		case mouseKeyAcceleration:
			m.motionAcceleration()
		case mouseKeyThreshold:
			m.motionThreshold()
		case mouseKeyScaling:
			m.motionScaling()
		case mouseKeyDoubleClick:
			m.doubleClick()
		case mouseKeyDragThreshold:
			m.dragThreshold()
		}
	})
}

func (tp *TrackPoint) handleGSettings() {
	tp.setting.Connect("changed", func(s *gio.Settings, key string) {
		switch key {
		case trackPointKeyMidButton:
			tp.enableMiddleButton()
		case trackPointKeyMidButtonTimeout:
			tp.middleButtonTimeout()
		case trackPointKeyWheel:
			tp.enableWheelEmulation()
		case trackPointKeyWheelButton:
			tp.wheelEmulationButton()
		case trackPointKeyWheelTimeout:
			tp.wheelEmulationTimeout()
		case trackPointKeyWheelHorizScroll:
			tp.enableWheelHorizScroll()
		case trackPointKeyLeftHanded:
			tp.enableLeftHanded()
		case trackPointKeyAcceleration:
			tp.motionAcceleration()
		case trackPointKeyThreshold:
			tp.motionThreshold()
		case trackPointKeyScaling:
			tp.motionScaling()
		}
	})
}

func (tpad *Touchpad) handleGSettings() {
	tpad.setting.Connect("changed", func(s *gio.Settings, key string) {
		switch key {
		case tpadKeyEnabled:
			tpad.enable(tpad.TPadEnable.Get())
		case tpadKeyLeftHanded:
			tpad.enableLeftHanded()
			tpad.enableTapToClick()
		case tpadKeyTapClick:
			tpad.enableTapToClick()
		case tpadKeyNaturalScroll:
			tpad.enableNaturalScroll()
		case tpadKeyScrollDelta:
			tpad.setScrollDistance()
		case tpadKeyEdgeScroll:
			tpad.enableEdgeScroll()
		case tpadKeyVertScroll, tpadKeyHorizScroll:
			tpad.enableTwoFingerScroll()
		case tpadKeyWhileTyping:
			tpad.disableWhileTyping()
		case tpadKeyAcceleration:
			tpad.motionAcceleration()
		case tpadKeyThreshold:
			tpad.motionThreshold()
		case tpadKeyScaling:
			tpad.motionScaling()
		case tpadKeyPalmDetect:
			tpad.enablePalmDetect()
		case tpadKeyPalmMinWidth, tpadKeyPalmMinZ:
			tpad.setPalmDimensions()
		}
	})
}

func (w *Wacom) handleGSettings() {
	w.setting.Connect("changed", func(s *gio.Settings, key string) {
		logger.Debugf("wacom gsettings changed %v", key)
		switch key {
		case wacomKeyLeftHanded:
			w.enableLeftHanded()
		case wacomKeyCursorMode:
			w.enableCursorMode()
		case wacomKeySuppress:
			w.setSuppress()
		case wacomKeyForceProportions:
			w.setArea()
		}
	})

	w.stylusSetting.Connect("changed", func(s *gio.Settings, key string) {
		logger.Debugf("wacom.stylus gsettings changed %v", key)
		switch key {
		case wacomKeyPressureSensitive:
			w.setPressureSensitiveForType(dxinput.WacomTypeStylus)
		case wacomKeyUpAction:
			w.setStylusButtonAction(btnNumUpKey, w.KeyUpAction.Get())
		case wacomKeyDownAction:
			w.setStylusButtonAction(btnNumDownKey, w.KeyDownAction.Get())
		case wacomKeyThreshold:
			w.setThresholdForType(dxinput.WacomTypeStylus)
		case wacomKeyRawSample:
			w.setRawSampleForType(dxinput.WacomTypeStylus)
		}
	})

	w.eraserSetting.Connect("changed", func(s *gio.Settings, key string) {
		logger.Debugf("wacom.eraser gsettings changed %v", key)
		switch key {
		case wacomKeyPressureSensitive:
			w.setPressureSensitiveForType(dxinput.WacomTypeEraser)
		case wacomKeyThreshold:
			w.setThresholdForType(dxinput.WacomTypeEraser)
		case wacomKeyRawSample:
			w.setRawSampleForType(dxinput.WacomTypeEraser)
		}
	})
}
