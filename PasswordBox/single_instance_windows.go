//go:build windows

package main

import (
	"errors"

	"golang.org/x/sys/windows"
)

var mutexHandle windows.Handle

func acquireSingleInstance() error {
	name, _ := windows.UTF16PtrFromString("PasswordBox_SingleInstance_Mutex")
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		return err
	}
	if windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
		windows.CloseHandle(handle)
		return errors.New("PasswordBox 已在运行")
	}
	mutexHandle = handle
	return nil
}
