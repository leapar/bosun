package conf

import (
	"https://github.com/leapar/bosun/cmd/bosun/search"
	"https://github.com/leapar/bosun/models"
	"https://github.com/leapar/bosun/opentsdb"
)

// TODO: remove this and merge it with Lookup
type ExprLookup struct {
	Tags    []string
	Entries []*ExprEntry
}

type ExprEntry struct {
	AlertKey models.AlertKey
	Values   map[string]string
}

func (lookup *ExprLookup) Get(key string, tag opentsdb.TagSet) (value string, ok bool) {
	for _, entry := range lookup.Entries {
		value, ok = entry.Values[key]
		if !ok {
			continue
		}
		match := true
		for ak, av := range entry.AlertKey.Group() {
			matches, err := search.Match(av, []string{tag[ak]})
			if err != nil {
				return "", false
			}
			if len(matches) == 0 {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		return
	}
	return "", false
}
