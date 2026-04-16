//go:build windows

package main

import (
	"context"
	"errors"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

var mutexHandle windows.Handle

func acquireSingleInstance() error {
	name, _ := windows.UTF16PtrFromString("PasswordBox_SingleInstance_Mutex")
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			_ = notifyShowWindow()
			return errors.New("PasswordBox 已在运行")
		}
		return err
	}
	if windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
		_ = notifyShowWindow()
		windows.CloseHandle(handle)
		return errors.New("PasswordBox 已在运行")
	}
	mutexHandle = handle
	return nil
}

func notifyShowWindow() error {
	name, _ := windows.UTF16PtrFromString("PasswordBox_ShowWindow_Event")
	handle, err := windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, name)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	return windows.SetEvent(handle)
}

func startShowWindowListener(ctx context.Context) {
	name, _ := windows.UTF16PtrFromString("PasswordBox_ShowWindow_Event")
	handle, err := windows.CreateEvent(nil, 0, 0, name)
	if err != nil {
		return
	}
	defer windows.CloseHandle(handle)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		ret, _ := windows.WaitForSingleObject(handle, 500)
		if ret == windows.WAIT_OBJECT_0 {
			runtime.Show(ctx)
		}
	}
}
