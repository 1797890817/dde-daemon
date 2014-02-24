/**
 * Copyright (c) 2011 ~ 2013 Deepin, Inc.
 *               2011 ~ 2013 jouyouyun
 *
 * Author:      jouyouyun <jouyouwen717@gmail.com>
 * Maintainer:  jouyouyun <jouyouwen717@gmail.com>
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

package main

import (
        //freedbus "dbus/org/freedesktop/dbus"
        polkit "dbus/org/freedesktop/policykit1"
        "dlib/glib-2.0"
        "fmt"
        "os"
        "os/exec"
        "strings"
)

var (
        genId = func() func() uint32 {
                id := uint32(0)
                return func() uint32 {
                        tmp := id
                        id += 1
                        return tmp
                }
        }()
)

const (
        POLKIT_DEST = "org.freedesktop.PolicyKit1"
        POLKIT_PATH = "/org/freedesktop/PolicyKit1/Authority"
        POLKIT_IFC  = "org.freedesktop.PolicyKit1.Authority"
)

func execCommand(cmdline string, args []string) {
        err := exec.Command(cmdline, args...).Run()
        if err != nil {
                fmt.Println("Exec", cmdline, args, "failed:", err)
                panic(err)
        }
}

func getBaseName(path string) string {
        strs := strings.Split(path, "/")
        return strs[len(strs)-1]
}

func fileIsExist(file string) bool {
        if _, err := os.Stat(file); os.IsExist(err) {
                fmt.Printf("'%s' is not exist\n", file)
                return false
        }
        //fmt.Printf("'%s' exist\n", file)

        return true
}

func isElementExist(element string, list []string) bool {
        for _, v := range list {
                if v == element {
                        return true
                }
        }

        return false
}

/*
 * To determine whether the character is [A-Za-z0-9]
 */
func charIsAlNum(ch byte) bool {
        if (ch >= '0' && ch <= '9') ||
                (ch >= 'a' && ch <= 'z') ||
                (ch >= 'A' && ch <= 'Z') {
                return true
        }

        return false
}

func readKeyFileValue(filename, group, key string, t int32) (interface{}, bool) {
        if !fileIsExist(filename) {
                return nil, false
        }

        keyFile := glib.NewKeyFile()
        defer keyFile.Free()
        ok, _ := keyFile.LoadFromFile(filename, glib.KeyFileFlagsKeepComments)
        if !ok {
                fmt.Printf("LoadKeyFile '%s' failed\n", filename)
                return nil, false
        }

        switch t {
        case KEY_TYPE_BOOL:
                v, err := keyFile.GetBoolean(group, key)
                if err != nil {
                        fmt.Printf("Get '%s' from '%s' failed: %s\n",
                                key, filename, err)
                        break
                }
                return v, true
        case KEY_TYPE_INT:
                v, err := keyFile.GetInteger(group, key)
                if err != nil {
                        fmt.Printf("Get '%s' from '%s' failed: %s\n",
                                key, filename, err)
                        break
                }
                return v, true
        case KEY_TYPE_STRING:
                v, err := keyFile.GetString(group, key)
                if err != nil {
                        fmt.Printf("Get '%s' from '%s' failed: %s\n",
                                key, filename, err)
                        break
                }
                return v, true
        }

        return nil, false
}

func writeKeyFileValue(filename, group, key string, t int32, value interface{}) {
        if !fileIsExist(filename) {
                return
        }

        keyFile := glib.NewKeyFile()
        defer keyFile.Free()
        ok, _ := keyFile.LoadFromFile(filename, glib.KeyFileFlagsKeepComments)
        if !ok {
                fmt.Printf("LoadKeyFile '%s' failed\n", filename)
                return
        }

        switch t {
        case KEY_TYPE_BOOL:
                keyFile.SetBoolean(group, key, value.(bool))
        case KEY_TYPE_INT:
                keyFile.SetInteger(group, key, value.(int))
        case KEY_TYPE_STRING:
                keyFile.SetString(group, key, value.(string))
        }

        _, contents, err := keyFile.ToData()
        if err != nil {
                fmt.Printf("KeyFile '%s' ToData failed: %s\n", filename, err)
                panic(err)
        }

        writeKeyFile(contents, filename)
}

func writeKeyFile(contents, file string) {
        if len(file) <= 0 {
                return
        }

        fmt.Println(contents)
        //return
        f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE, ETC_PERM)
        if err != nil {
                fmt.Printf("OpenFile '%s' failed: %s\n", file, err)
                panic(err)
        }
        defer f.Close()

        _, err = f.WriteString(contents)
        if err != nil {
                fmt.Printf("Write in '%s' failed: %s\n", file, err)
                panic(err)
        }
}

type polkitSubject struct {
        /*
         * The following kinds of subjects are known:
         * Unix Process: should be set to unix-process with keys
         *                  pid (of type uint32) and
         *                  start-time (of type uint64)
         * Unix Session: should be set to unix-session with the key
         *                  session-id (of type string)
         * System Bus Name: should be set to system-bus-name with the key
         *                  name (of type string)
         */
        subjectKind    string
        subjectDetails map[string]interface{}
}

func authWithPolkit(actionId string) {
        var (
                objPolkit *polkit.Authority
                //objDbus   *freedbus.DBusDaemon
                err     error
        )

        objPolkit, err = polkit.NewAuthority(POLKIT_PATH)
        if err != nil {
                fmt.Println("New Authority Failed:", err)
                panic(err)
        }

        /*
           objDbus, err = freedbus.NewDBusDaemon("/")
           if err != nil {
                   fmt.Println("New DBusDaemon Failed:", err)
                   panic(err)
           }

           pid, err1 := objDbus.GetConnectionUnixProcessID(ACCOUNT_DEST)
           if err1 != nil {
                   fmt.Println("GetConnectionUnixProcessID Failed:", err1)
                   panic(err1)
           }
        */
        pid := os.Getpid()
        subject := polkitSubject{}
        subject.subjectKind = "unix-process"
        subject.subjectDetails = make(map[string]interface{})
        subject.subjectDetails["pid"] = uint32(pid)
        subject.subjectDetails["start-time"] = uint64(0)
        details := make(map[string]string)
        flags := uint32(1)
        cancelId := fmt.Sprintf("%d", genId())

        infaces := []interface{}{}
        infaces = append(infaces, subject.subjectKind)
        infaces = append(infaces, subject.subjectDetails)
        _, err = objPolkit.CheckAuthorization(infaces, actionId, details, flags, cancelId)
        if err != nil {
                fmt.Println("CheckAuthorization Failed:", err)
                panic(err)
        }
}
