package collectors

import (
	"https://github.com/leapar/bosun/cmd/scollector/conf"
	"fmt"
)

func AddProcessConfig(params conf.ProcessParams) error {
	return fmt.Errorf("process watching not implemented on Darwin")
}

func WatchProcesses() {
}
