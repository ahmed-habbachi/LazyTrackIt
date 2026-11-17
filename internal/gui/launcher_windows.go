//go:build windows

package gui

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// installLauncher adds a "LazyTrackIt" shortcut to the user's Start Menu on
// the first run, unless one is already there (the installer makes the same
// one).
func installLauncher(exe string, firstRun bool) error {
	if !firstRun {
		return nil
	}
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return errors.New("APPDATA isn't set")
	}
	path := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "LazyTrackIt.lnk")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return createShortcut(path, exe)
}

// createShortcut writes a .lnk at path that starts exe, through the
// WScript.Shell COM object.
func createShortcut(path, exe string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		// S_FALSE: COM was already initialized on this thread; fine.
		var oleErr *ole.OleError
		if !errors.As(err, &oleErr) || oleErr.Code() != 1 {
			return err
		}
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return err
	}
	defer unknown.Release()
	shell, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer shell.Release()

	res, err := oleutil.CallMethod(shell, "CreateShortcut", path)
	if err != nil {
		return err
	}
	link := res.ToIDispatch()
	defer link.Release()
	for name, value := range map[string]string{
		"TargetPath":       exe,
		"WorkingDirectory": filepath.Dir(exe),
		"IconLocation":     exe + ",0",
		"Description":      "Log your work time and check in or out",
	} {
		if _, err := oleutil.PutProperty(link, name, value); err != nil {
			return err
		}
	}
	_, err = oleutil.CallMethod(link, "Save")
	return err
}
