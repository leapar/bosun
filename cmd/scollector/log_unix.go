// +build !windows,!nacl,!plan9

package main

import (
	"github.com/leapar/bosun/_version"
	"github.com/leapar/bosun/slog"
)

func init() {
	err := slog.SetSyslog("scollector")
	if err != nil {
		slog.Error(err)
	}
	slog.Infof("starting %s", version.GetVersionInfo("scollector"))
}
