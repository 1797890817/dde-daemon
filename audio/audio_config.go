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

package audio

import (
	"time"

	"pkg.deepin.io/lib/pulse"
)

func (a *Audio) applyConfig() {
	info, err := readConfig()
	if err != nil {
		logger.Warning("Read config info failed:", err)
		return
	}

	if !a.isConfigValid(info) {
		logger.Warning("Invalid config:", info.string())
		a.trySelectBestPort()
		return
	}

	for _, card := range a.ctx.GetCardList() {
		v, ok := info.Profiles[card.Name]
		if !ok {
			continue
		}

		if card.ActiveProfile.Name != v {
			card.SetProfile(v)
			time.Sleep(time.Microsecond * 300)
		}
	}

	var sinkValidity = true
	for _, s := range a.ctx.GetSinkList() {
		if s.Name == info.Sink {
			if len(info.SinkPort) == 0 {
				sinkValidity = false
				break
			}
			port := pulse.PortInfos(s.Ports).Get(info.SinkPort)
			// if port invalid, nothing to do.
			// TODO: some device port can play sound when state is 'NO', how to fix?
			if port == nil {
				sinkValidity = false
				break
			}

			if s.ActivePort.Name != info.SinkPort {
				a.ctx.SetSinkPortByIndex(s.Index, info.SinkPort)
				time.Sleep(time.Microsecond * 50)
			}
			cv := s.Volume.SetAvg(info.SinkVolume)
			a.ctx.SetSinkVolumeByIndex(s.Index, cv)
			time.Sleep(time.Microsecond * 50)
			break
		}
	}
	logger.Debug("Audio config sink validity:", sinkValidity, info.Sink)
	if sinkValidity {
		a.ctx.SetDefaultSink(info.Sink)
		time.Sleep(time.Microsecond * 50)
	}

	var sourceValidity = true
	for _, s := range a.ctx.GetSourceList() {
		if s.Name == info.Source {
			if len(info.SourcePort) == 0 {
				sourceValidity = false
				continue
			}
			port := pulse.PortInfos(s.Ports).Get(info.SourcePort)
			if port == nil {
				sourceValidity = false
				continue
			}
			if s.ActivePort.Name != info.SourcePort {
				a.ctx.SetSourcePortByIndex(s.Index, info.SourcePort)
				time.Sleep(time.Microsecond * 50)
			}
			cv := s.Volume.SetAvg(info.SourceVolume)
			a.ctx.SetSourceVolumeByIndex(s.Index, cv)
			time.Sleep(time.Microsecond * 50)
			break
		}
	}
	logger.Debug("Audio config source validity:", sourceValidity, info.Source)
	if sourceValidity {
		a.ctx.SetDefaultSource(info.Source)
		time.Sleep(time.Microsecond * 50)
	}
}

func (a *Audio) trySelectBestPort() {
	sinkId, sinkPort := a.cards.getAvailablePort(pulse.DirectionSink)
	if sinkPort.Name != "" {
		logger.Debugf("Will switch to sink: %#v", sinkPort)
		err := a.SetPort(sinkId, sinkPort.Name, int32(sinkPort.Direction))
		if err != nil {
			logger.Warningf("Failed to switch to sink port: %#v, error: %v", sinkPort, err)
		}
	}

	sourceId, sourcePort := a.cards.getAvailablePort(pulse.DirectionSource)
	if sourcePort.Name != "" && sourceId == sinkId {
		logger.Debugf("Will switch to source: %#v", sourcePort)
		err := a.SetPort(sourceId, sourcePort.Name, int32(sourcePort.Direction))
		if err != nil {
			logger.Warningf("Failed to switch to source port: %#v, error: %v", sourcePort, err)
		}
	}
}

func (a *Audio) saveConfig() {
	a.saverLocker.Lock()
	if a.isSaving {
		a.saverLocker.Unlock()
		return
	}

	a.isSaving = true
	a.saverLocker.Unlock()

	time.AfterFunc(time.Second*1, func() {
		a.doSaveConfig()

		a.saverLocker.Lock()
		a.isSaving = false
		a.saverLocker.Unlock()
	})
}

func (a *Audio) doSaveConfig() {
	var info = config{
		Profiles: make(map[string]string),
	}

	ctx := a.context()
	for _, card := range ctx.GetCardList() {
		info.Profiles[card.Name] = card.ActiveProfile.Name
	}

	for _, sinkInfo := range ctx.GetSinkList() {
		if a.getDefaultSinkName() != sinkInfo.Name {
			continue
		}

		info.Sink = sinkInfo.Name
		info.SinkPort = sinkInfo.ActivePort.Name
		info.SinkVolume = sinkInfo.Volume.Avg()
		break
	}

	for _, sourceInfo := range ctx.GetSourceList() {
		if a.getDefaultSourceName() != sourceInfo.Name {
			continue
		}

		info.Source = sourceInfo.Name
		info.SourcePort = sourceInfo.ActivePort.Name
		info.SourceVolume = sourceInfo.Volume.Avg()
		break
	}

	err := saveConfig(&info)
	if err != nil {
		logger.Warning("Save config file failed:", info.string(), err)
	}
}

func (a *Audio) isConfigValid(cfg *config) bool {
	if len(cfg.Profiles) == 0 {
		return false
	}

	// check cfg.Profiles
	var validProfileCount int
	for _, card := range a.ctx.GetCardList() {
		cardProfile, ok := cfg.Profiles[card.Name]
		if !ok {
			continue
		}
		// find cardProfile in card.Profiles
		var found bool
		for _, profile := range card.Profiles {
			if profile.Name == cardProfile {
				found = true
				break
			}
		}

		if found {
			validProfileCount++
		} else {
			// cardProfile is invalid
			return false
		}
	}
	if validProfileCount != len(cfg.Profiles) {
		return false
	}

	// check cfg.Sink and cfg.SinkPort
	var sinkValid bool
	for _, sink := range a.ctx.GetSinkList() {
		if sink.Name != cfg.Sink {
			continue
		}

		if len(cfg.SinkPort) == 0 {
			sinkValid = true
			break
		}

		for _, port := range sink.Ports {
			if port.Name == cfg.SinkPort {
				sinkValid = true
			}
		}
		break
	}
	if !sinkValid {
		return false
	}

	// check cfg.Source and cfg.SourcePort
	var sourceValid bool
	for _, source := range a.ctx.GetSourceList() {
		if source.Name != cfg.Source {
			continue
		}
		if len(cfg.SourcePort) == 0 {
			sourceValid = true
			break
		}

		for _, port := range source.Ports {
			if port.Name == cfg.SourcePort {
				sourceValid = true
			}
		}
		break
	}
	return sourceValid
}
