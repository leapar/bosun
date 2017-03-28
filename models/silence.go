package models

import (
	"crypto/sha1"
	"fmt"
	"time"

	"github.com/leapar/bosun/opentsdb"
	"github.com/leapar/bosun/util"
)

type Silence struct {
	Start, End time.Time
	Alert      string
	Tags       opentsdb.TagSet
	TagString  string
	Forget     bool
	User       string
	Message    string
}

func (s *Silence) Silenced(now time.Time, alert string, tags opentsdb.TagSet) bool {
	if !s.ActiveAt(now) {
		return false
	}
	return s.Matches(alert, tags)
}

func (s *Silence) ActiveAt(now time.Time) bool {
	if now.Before(s.Start) || now.After(s.End) {
		return false
	}
	return true
}

func (s *Silence) Matches(alert string, tags opentsdb.TagSet) bool {
	if s.Alert != "" && s.Alert != alert {
		return false
	}
	for k, pattern := range s.Tags {
		tagv, ok := tags[k]
		if !ok {
			return false
		}
		matched, _ := util.Match(pattern, tagv)
		if !matched {
			return false
		}
	}
	return true
}

func (s Silence) ID() string {
	h := sha1.New()
	fmt.Fprintf(h, "%s|%s|%s%s", s.Start, s.End, s.Alert, s.Tags)
	return fmt.Sprintf("%x", h.Sum(nil))
}
