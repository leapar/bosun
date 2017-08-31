package web

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/gorilla/mux"
	"github.com/leapar/bosun/opentsdb"
)

// UniqueMetrics returns a sorted list of available metrics.
func UniqueMetrics(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	q := r.URL.Query()
	var epoch int64
	if v := q.Get("since"); v != "" {
		var err error
		epoch, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			serveError(w, fmt.Errorf("could not convert since parameter (expecting epoch value): %v", err))
		}
	}
	uid := r.FormValue("uid")
	if uid == "" {
		uid = schedule.SystemConf.GetUid()
	}
	values, err := schedule.Search.UniqueMetrics(uid, epoch)
	if err != nil {
		return nil, err
	}
	// remove anything starting with double underscore.
	if v := q.Get("unfiltered"); v != "" {
		return values, nil
	}
	filtered := []string{}
	for _, v := range values {
		if len(v) < 2 || v[0:2] != "__" {
			filtered = append(filtered, v)
		}
	}
	return filtered, nil
}

func TagKeysByMetric(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	metric := vars["metric"]
	uid := r.FormValue("uid")
	if uid == "" {
		uid = schedule.SystemConf.GetUid()
	}
	return schedule.Search.TagKeysByMetric(metric, uid)
}

func TagValuesByMetricTagKey(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	metric := vars["metric"]
	tagk := vars["tagk"]
	since, err := getSince(r)
	if err != nil {
		return nil, err
	}
	uid := r.FormValue("uid")
	if uid == "" {
		uid = schedule.SystemConf.GetUid()
	}
	return schedule.Search.TagValuesByMetricTagKey(metric, tagk, uid, since)
}

func getSince(r *http.Request) (time.Duration, error) {
	s := r.FormValue("since")
	since := schedule.SystemConf.GetSearchSince()

	if s != "" && s != "default" {
		td, err := opentsdb.ParseDuration(s)
		if err != nil {
			return 0, err
		}
		since = time.Duration(td)
	}
	return since, nil
}

func FilteredTagsetsByMetric(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	metric := vars["metric"]
	tagset := opentsdb.TagSet{}
	var err error
	ts := r.FormValue("tags")
	if ts != "" {
		if tagset, err = opentsdb.ParseTags(ts); err != nil {
			return nil, err
		}
	}

	since := int64(0)
	sinceStr := r.FormValue("since")
	if sinceStr != "" {
		since, err = strconv.ParseInt(sinceStr, 10, 64) //since will be set to 0 again in case of errors
		if err != nil {
			return nil, err
		}
	}
	uid := r.FormValue("uid")
	if uid == "" {
		uid = schedule.SystemConf.GetUid()
	}
	return schedule.Search.FilteredTagSets(metric, uid, tagset, since)
}

func MetricsByTagPair(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	since, err := getSince(r)
	if err != nil {
		return nil, err
	}
	tagk := vars["tagk"]
	tagv := vars["tagv"]
	uid := r.FormValue("uid")
	if uid == "" {
		uid = schedule.SystemConf.GetUid()
	}
	return schedule.Search.MetricsByTagPair(tagk, tagv, uid, since)
}

func MetricsByTagKey(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	tagk := vars["tagk"]
	since, err := getSince(r)
	if err != nil {
		return nil, err
	}
	uid := r.FormValue("uid")
	if uid == "" {
		uid = schedule.SystemConf.GetUid()
	}
	tagValues, err := schedule.Search.TagValuesByTagKey(tagk, uid, since)
	if err != nil {
		return nil, err
	}
	// map[tagv][metrics...]
	tagvMetrics := make(map[string][]string)
	for _, tagv := range tagValues {
		metrics, err := schedule.Search.MetricsByTagPair(tagk, tagv, uid, since)
		if err != nil {
			return nil, err
		}
		tagvMetrics[tagv] = metrics
	}
	return tagvMetrics, nil
}

func TagValuesByTagKey(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	tagk := vars["tagk"]
	since, err := getSince(r)
	if err != nil {
		return nil, err
	}
	uid := r.FormValue("uid")
	if uid == "" {
		uid = schedule.SystemConf.GetUid()
	}
	return schedule.Search.TagValuesByTagKey(tagk, uid, since)
}
