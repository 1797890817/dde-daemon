package main

import (
	"flag"
	"os/exec"

	"dbus/com/deepin/sessionmanager"

	"gir/gio-2.0"
	"pkg.deepin.io/lib/appinfo/desktopappinfo"
	"pkg.deepin.io/lib/log"
)

var logger = log.NewLogger("cmd/default-terminal")

var launchAppFlag bool
var executeFlag string

const (
	gsSchemaDefaultTerminal = "com.deepin.desktop.default-applications.terminal"
	gsKeyAppId              = "app-id"
	gsKeyExec               = "exec"
	gsKeyExecArg            = "exec-arg"
)

func init() {
	flag.BoolVar(&launchAppFlag, "launch-app", false,
		"launch via startdde LaunchApp")
	flag.StringVar(&executeFlag, "e", "", "run a program in the terminal")
}

func main() {
	flag.Parse()

	settings := gio.NewSettings(gsSchemaDefaultTerminal)
	defer settings.Unref()

	if launchAppFlag {
		appId := settings.GetString(gsKeyAppId)
		appInfo := desktopappinfo.NewDesktopAppInfo(appId)

		if appInfo != nil {
			startManager, err := sessionmanager.NewStartManager("com.deepin.SessionManager",
				"/com/deepin/StartManager")
			if err != nil {
				panic(err)
			}
			filename := appInfo.GetFileName()
			err = startManager.LaunchApp(filename, 0, nil)
			sessionmanager.DestroyStartManager(startManager)

			if err != nil {
				logger.Warning(err)
			}
		} else {
			runFallbackTerm()
		}

	} else {
		termExec := settings.GetString(gsKeyExec)
		termExecArg := settings.GetString(gsKeyExecArg)
		termPath, _ := exec.LookPath(termExec)
		if termPath == "" {
			// try again
			termExecArg = "-e"
			termPath = getTerminalPath()
			if termPath == "" {
				logger.Fatal("failed to get terminal path")
			}
		}

		var args []string
		if executeFlag != "" {
			args = []string{termExecArg, executeFlag}
		}

		err := exec.Command(termPath, args...).Run()
		if err != nil {
			logger.Warning(err)
		}
	}
}

func runFallbackTerm() {
	termPath := getTerminalPath()
	if termPath == "" {
		logger.Warning("failed to get terminal path")
		return
	}
	err := exec.Command(termPath).Run()
	if err != nil {
		logger.Warning(err)
	}
}

var terms = []string{
	"deepin-terminal",
	"gnome-terminal",
	"terminator",
	"xfce4-terminal",
	"rxvt",
	"xterm",
}

func getTerminalPath() string {
	for _, exe := range terms {
		file, _ := exec.LookPath(exe)
		if file != "" {
			return file
		}
	}
	return ""
}
