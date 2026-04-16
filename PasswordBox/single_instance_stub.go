//go:build !windows

package main

import "context"

func acquireSingleInstance() error {
	return nil
}

func startShowWindowListener(ctx context.Context) {
}
