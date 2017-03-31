package ping // import "github.com/leapar/bosun/cmd/bosun/ping"

import (
	"net"
	"time"

	fastping "github.com/tatsushid/go-fastping"

	"github.com/leapar/bosun/cmd/bosun/search"
	"github.com/leapar/bosun/collect"
	"github.com/leapar/bosun/metadata"
	"github.com/leapar/bosun/opentsdb"
	"github.com/leapar/bosun/slog"
)

func init() {
	metadata.AddMetricMeta("bosun.ping.resolved", metadata.Gauge, metadata.Bool,
		"1=Ping resolved to an IP Address. 0=Ping failed to resolve to an IP Address.")
	metadata.AddMetricMeta("bosun.ping.rtt", metadata.Gauge, metadata.MilliSecond,
		"The number of milliseconds for the echo reply to be received. Also known as Round Trip Time.")
	metadata.AddMetricMeta("bosun.ping.timeout", metadata.Gauge, metadata.Ok,
		"0=Ping responded before timeout. 1=Ping did not respond before 5 second timeout.")
}

const pingFreq = time.Second * 15

// PingHosts pings all hosts that bosun has indexed as recently as the PingDuration
// provided via the systemConf
func PingHosts(search *search.Search, uid string, duration time.Duration) {
	for range time.Tick(pingFreq) {
		hosts, err := search.TagValuesByTagKey("host", uid, duration)
		if err != nil {
			slog.Error(err)
			continue
		}
		for _, host := range hosts {
			go pingHost(host)
		}
	}
}

func pingHost(host string) {
	p := fastping.NewPinger()
	tags := opentsdb.TagSet{"dst_host": host}
	resolved := 0
	defer func() {
		collect.Put("ping.resolved", tags, resolved)
	}()
	ra, err := net.ResolveIPAddr("ip4:icmp", host)
	if err != nil {
		return
	}
	resolved = 1
	p.AddIPAddr(ra)
	p.MaxRTT = time.Second * 5
	timeout := 1
	p.OnRecv = func(addr *net.IPAddr, t time.Duration) {
		collect.Put("ping.rtt", tags, float64(t)/float64(time.Millisecond))
		timeout = 0
	}
	if err := p.Run(); err != nil {
		slog.Errorln(err)
	}
	collect.Put("ping.timeout", tags, timeout)
}
