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

package main

import (
	"os"

	// modules:
	_ "pkg.deepin.io/dde/daemon/accounts"
	_ "pkg.deepin.io/dde/daemon/apps"
	_ "pkg.deepin.io/dde/daemon/system/gesture"
	_ "pkg.deepin.io/dde/daemon/system/power"
	_ "pkg.deepin.io/dde/daemon/system/swapsched"
	_ "pkg.deepin.io/dde/daemon/system/timedated"

	"gir/glib-2.0"
	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/dbusutil"
	. "pkg.deepin.io/lib/gettext"
	"pkg.deepin.io/lib/log"
)

type Daemon struct{}

const (
	dbusServiceName = "com.deepin.daemon.Daemon"
	dbusPath        = "/com/deepin/daemon/Daemon"
	dbusInterface   = dbusServiceName
)

var logger = log.NewLogger("daemon/dde-system-daemon")
var _daemon *Daemon

func main() {
	logger.BeginTracing()
	defer logger.EndTracing()

	service, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Fatal("failed to new system service", err)
	}

	hasOwner, err := service.NameHasOwner(dbusServiceName)
	if err != nil {
		logger.Fatal("failed to call NameHasOwner:", err)
	}
	if hasOwner {
		logger.Fatalf("name %q already has the owner", dbusServiceName)
	}

	// fix no PATH when was launched by dbus
	if os.Getenv("PATH") == "" {
		logger.Warning("No PATH found, manual special")
		os.Setenv("PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	}

	InitI18n()
	Textdomain("dde-daemon")

	logger.SetRestartCommand("/usr/lib/deepin-daemon/dde-system-daemon")

	_daemon = &Daemon{}
	err = service.Export(_daemon)
	if err != nil {
		logger.Fatal("failed to export:", err)
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		logger.Fatal("failed to request name:", err)
	}

	loader.SetService(service)
	loader.StartAll()
	defer loader.StopAll()

	// NOTE: system/power module requires glib loop
	go glib.StartLoop()

	service.Wait()
}

func (*Daemon) GetDBusExportInfo() dbusutil.ExportInfo {
	return dbusutil.ExportInfo{
		Path:      dbusPath,
		Interface: dbusInterface,
	}
}
