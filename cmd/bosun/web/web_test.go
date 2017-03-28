package web

import (
	"testing"
	"time"

	"github.com/leapar/bosun/cmd/bosun/conf"
	"github.com/leapar/bosun/cmd/bosun/conf/rule"
)

func TestErrorTemplate(t *testing.T) {
	c, err := rule.NewConf("", conf.EnabledBackends{}, `
		template t {
			body = {{.Eval "invalid"}}
		}
		alert a {
			template = t
			crit = 1
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = procRule(nil, c, c.Alerts["a"], time.Time{}, false, "", "")
	if err != nil {
		t.Fatal(err)
	}
}
