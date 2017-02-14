package keybinding

import (
	"errors"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgb/xtest"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/keybind"
)

type NumLockState uint

const (
	NumLockOff NumLockState = iota
	NumLockOn
	NumLockUnknown
)

func queryNumLockState(xu *xgbutil.XUtil) (NumLockState, error) {
	queryPointerReply, err := xproto.QueryPointer(xu.Conn(), xu.RootWin()).Reply()
	if err != nil {
		return NumLockUnknown, err
	}
	logger.Debugf("query pointer reply %#v", queryPointerReply)
	on := queryPointerReply.Mask&xproto.ModMask2 != 0
	if on {
		return NumLockOn, nil
	} else {
		return NumLockOff, nil
	}
}

func setNumLockState(xu *xgbutil.XUtil, state NumLockState) error {
	if !(state == NumLockOff || state == NumLockOn) {
		return errors.New("invalid numlock state")
	}

	state0, err := queryNumLockState(xu)
	if err != nil {
		return err
	}

	if state0 != state {
		return changeNumLockState(xu)
	}
	return nil
}

func changeNumLockState(xu *xgbutil.XUtil) (err error) {
	// get Num_Lock keycode
	_, codes, _ := keybind.ParseString(xu, "Num_Lock")
	if len(codes) == 0 {
		return errors.New("get Num_Lock keycode failed")
	}
	numLockKeycode := byte(codes[0])
	logger.Debug("numLockKeycode is", numLockKeycode)

	x := xu.Conn()
	root := xu.RootWin()

	// fake key press
	err = xtest.FakeInputChecked(x, xproto.KeyPress, numLockKeycode, xproto.TimeCurrentTime, root, 0, 0, 0).Check()
	if err != nil {
		return err
	}
	// fake key release
	err = xtest.FakeInputChecked(x, xproto.KeyRelease, numLockKeycode, xproto.TimeCurrentTime, root, 0, 0, 0).Check()
	if err != nil {
		return err
	}
	return nil
}
