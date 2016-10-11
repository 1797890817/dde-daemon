package dock

import (
	"strconv"
	"strings"
)

type IdentifyWindowFunc struct {
	Name string
	Fn   _IdentifyWindowFunc
}

type _IdentifyWindowFunc func(*DockManager, *WindowInfo) (string, *AppInfo)

func (m *DockManager) registerIdentifyWindowFuncs() {
	m.registerIdentifyWindowFunc("PidEnv", identifyWindowByPidEnv)
	m.registerIdentifyWindowFunc("Rule", identifyWindowByRule)
	m.registerIdentifyWindowFunc("Bamf", identifyWindowByBamf)
	m.registerIdentifyWindowFunc("Pid", identifyWindowByPid)
	m.registerIdentifyWindowFunc("Cache", identifyWindowByCache)
	m.registerIdentifyWindowFunc("GtkAppId", identifyWindowByGtkAppId)
	m.registerIdentifyWindowFunc("WmClass", identifyWindowByWmClass)
}

func (m *DockManager) registerIdentifyWindowFunc(name string, fn _IdentifyWindowFunc) {
	m.identifyWindowFuns = append(m.identifyWindowFuns, &IdentifyWindowFunc{
		Name: name,
		Fn:   fn,
	})
}

func (m *DockManager) identifyWindow(winInfo *WindowInfo) (string, *AppInfo) {
	logger.Debugf("identifyWindow: window id: %v, window innerId: %q", winInfo.window, winInfo.innerId)
	if winInfo.innerId == "" {
		logger.Debug("identifyWindow: failed winInfo no innerId")
		return "", nil
	}

	for idx, item := range m.identifyWindowFuns {
		name := item.Name
		logger.Debugf("identifyWindow: try %s:%d", name, idx)
		innerId, appInfo := item.Fn(m, winInfo)
		if innerId != "" {
			// success
			logger.Debugf("identifyWindow by %s success, innerId: %q, appInfo: %v", name, innerId, appInfo)
			return innerId, appInfo
		}
	}
	// fail
	logger.Debugf("identifyWindow: failed")
	return winInfo.innerId, nil
}

func identifyWindowByCache(m *DockManager, winInfo *WindowInfo) (string, *AppInfo) {
	desktopHash := m.desktopWindowsMapCacheManager.GetKeyByValue(winInfo.innerId)
	logger.Debugf("identifyWindowByCache: desktop hash: %q", desktopHash)
	var appInfo *AppInfo
	if desktopHash != "" {
		appInfo = m.desktopHashFileMapCacheManager.GetAppInfo(desktopHash)
		if appInfo != nil {
			// success
			return appInfo.innerId, appInfo
		} else {
			// cache fail
			logger.Debug("identifyWindowByCache: cache fail")
			m.desktopHashFileMapCacheManager.DeleteKey(desktopHash)
			m.desktopWindowsMapCacheManager.DeleteKeyValue(desktopHash, winInfo.innerId)
		}
	}
	// fail
	return "", nil
}

func identifyWindowByPid(m *DockManager, winInfo *WindowInfo) (string, *AppInfo) {
	if winInfo.pid != 0 {
		logger.Debugf("identifyWindowByPid: pid: %d", winInfo.pid)
		entry := m.Entries.GetByWindowPid(winInfo.pid)
		if entry != nil {
			// success
			return entry.innerId, entry.appInfo
		}
	}
	// fail
	return "", nil
}

func identifyWindowByGtkAppId(m *DockManager, winInfo *WindowInfo) (string, *AppInfo) {
	gtkAppId := winInfo.gtkAppId
	logger.Debugf("identifyWindowByGtkAppId: gtkAppId: %q", gtkAppId)
	if gtkAppId != "" {
		appInfo := NewAppInfo(gtkAppId)
		if appInfo != nil {
			// success
			return appInfo.innerId, appInfo
		}
	}
	// fail
	return "", nil
}

func identifyWindowByPidEnv(m *DockManager, winInfo *WindowInfo) (string, *AppInfo) {
	pid := winInfo.pid
	process := winInfo.process
	if process != nil && pid != 0 {
		launchedDesktopFile := process.environ["GIO_LAUNCHED_DESKTOP_FILE"]
		launchedDesktopFilePid, _ := strconv.ParseUint(
			process.environ["GIO_LAUNCHED_DESKTOP_FILE_PID"], 10, 32)

		logger.Debugf("identifyWindowByPidEnv: launchedDesktopFile: %q, pid: %d",
			launchedDesktopFile, launchedDesktopFilePid)

		if uint(launchedDesktopFilePid) == pid {
			appInfo := NewAppInfoFromFile(launchedDesktopFile)
			if appInfo != nil {
				// success
				return appInfo.innerId, appInfo
			}
		}
	}
	// fail
	return "", nil
}

func identifyWindowByRule(m *DockManager, winInfo *WindowInfo) (string, *AppInfo) {
	ret := m.windowPatterns.Match(winInfo)
	if ret == "" {
		return "", nil
	}
	logger.Debug("identifyWindowByRule ret:", ret)
	// parse ret
	// id=$appId or env
	var appInfo *AppInfo
	if len(ret) > 4 && strings.HasPrefix(ret, "id=") {
		appInfo = NewAppInfo(ret[3:])
	} else if ret == "env" {
		process := winInfo.process
		if process != nil {
			launchedDesktopFile := process.environ["GIO_LAUNCHED_DESKTOP_FILE"]
			if launchedDesktopFile != "" {
				appInfo = NewAppInfoFromFile(launchedDesktopFile)
			}
		}
	}

	if appInfo != nil {
		return appInfo.innerId, appInfo
	}
	return "", nil
}

func identifyWindowByWmClass(m *DockManager, winInfo *WindowInfo) (string, *AppInfo) {
	if winInfo.wmClass != nil {
		instance := winInfo.wmClass.Instance
		if instance != "" {
			appInfo := NewAppInfo(instance)
			if appInfo != nil {
				return appInfo.innerId, appInfo
			}
		}

		class := winInfo.wmClass.Class
		if class != "" {
			appInfo := NewAppInfo(class)
			if appInfo != nil {
				return appInfo.innerId, appInfo
			}
		}
	}
	// fail
	return "", nil
}

func identifyWindowByBamf(m *DockManager, winInfo *WindowInfo) (string, *AppInfo) {
	// bamf
	win := winInfo.window
	desktop := getDesktopFromWindowByBamf(win)
	if desktop != "" {
		appInfo := NewAppInfoFromFile(desktop)
		if appInfo != nil {
			// success
			return appInfo.innerId, appInfo
		}
	}
	return "", nil
}
