//go:build !windows

package main

func acquireSingleInstance() error {
	return nil
}
