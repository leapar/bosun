// +build !windows,!nacl,!plan9

package main

import (
	"https://github.com/leapar/bosun/_version"
	"https://github.com/leapar/bosun/slog"
)

func init() {
	err := slog.SetSyslog("scollector")
	if err != nil {
		slog.Error(err)
	}
	slog.Infof("starting %s", version.GetVersionInfo("scollector"))
}
