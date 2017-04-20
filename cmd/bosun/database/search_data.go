package database

import (
	"fmt"
	"strconv"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/leapar/bosun/opentsdb"
	"github.com/leapar/bosun/slog"
	"github.com/leapar/bosun/util"
)

/*
Search data in redis:

Metrics by tags:
search:metrics:{tagk}={tagv} -> hash of metric name to timestamp

Tag keys by metric:
search:tagk:{metric} -> hash of tag key to timestamp

Tag Values By metric/tag key
search:tagv:{metric}:{tagk} -> hash of tag value to timestamp
metric "__all__" is a special key that will hold all values for the tag key, regardless of metric

All Metrics:
search:allMetrics -> hash of metric name to timestamp

search:mts:{metric} -> all tag sets for a metric. Hash with time stamps
*/

const Search_All = "__all__"

func searchAllMetricsKey(uid string) string {
	return fmt.Sprintf("search:allMetrics:%s", uid)
}
func searchMetricKey(tagK, tagV, uid string) string {
	return fmt.Sprintf("search:metrics:%s:%s=%s", uid, tagK, tagV)
}
func searchTagkKey(metric, uid string) string {
	return fmt.Sprintf("search:tagk:%s:%s", uid, metric)
}
func searchTagvKey(metric, tagK, uid string) string {
	return fmt.Sprintf("search:tagv:%s:%s:%s", uid, metric, tagK)
}
func searchMetricTagSetKey(metric, uid string) string {
	return fmt.Sprintf("search:mts:%s:%s", uid, metric)
}

func searchHostTagSetKey(host, uid string) string {
	return fmt.Sprintf("search:hts:%s:%s", uid, host)
}

func searchTagSetHostKey(tagK, tagV, uid string) string {
	return fmt.Sprintf("search:hosts:%s:%s=%s", uid, tagK, tagV)
}

func (d *dataAccess) Search() SearchDataAccess {
	return d
}

func (d *dataAccess) AddMetricForTag(tagK, tagV, metric, uid string, time int64) error {
	conn := d.Get()
	defer conn.Close()

	_, err := conn.Do("HSET", searchMetricKey(tagK, tagV, uid), metric, time)
	return slog.Wrap(err)
}

func (d *dataAccess) GetMetricsForTag(tagK, tagV, uid string) (map[string]int64, error) {
	conn := d.Get()
	defer conn.Close()

	return stringInt64Map(conn.Do("HGETALL", searchMetricKey(tagK, tagV, uid)))
}

func stringInt64Map(d interface{}, err error) (map[string]int64, error) {
	if err != nil {
		return nil, err
	}
	vals, err := redis.Strings(d, err)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64)
	for i := 1; i < len(vals); i += 2 {
		time, _ := strconv.ParseInt(vals[i], 10, 64)
		result[vals[i-1]] = time
	}
	return result, slog.Wrap(err)
}

func (d *dataAccess) AddTagKeyForMetric(metric, tagK, uid string, time int64) error {
	conn := d.Get()
	defer conn.Close()

	_, err := conn.Do("HSET", searchTagkKey(metric, uid), tagK, time)
	return slog.Wrap(err)
}

func (d *dataAccess) GetTagKeysForMetric(metric, uid string) (map[string]int64, error) {
	conn := d.Get()
	defer conn.Close()

	return stringInt64Map(conn.Do("HGETALL", searchTagkKey(metric, uid)))
}

func (d *dataAccess) AddMetric(metric, uid string, time int64) error {
	conn := d.Get()
	defer conn.Close()

	_, err := conn.Do("HSET", searchAllMetricsKey(uid), metric, time)
	return slog.Wrap(err)
}
func (d *dataAccess) GetAllMetrics(uid string) (map[string]int64, error) {
	conn := d.Get()
	defer conn.Close()

	return stringInt64Map(conn.Do("HGETALL", searchAllMetricsKey(uid)))
}

func (d *dataAccess) AddTagValue(metric, tagK, tagV, uid string, time int64) error {
	conn := d.Get()
	defer conn.Close()

	_, err := conn.Do("HSET", searchTagvKey(metric, tagK, uid), tagV, time)
	return slog.Wrap(err)
}

func (d *dataAccess) GetTagValues(metric, tagK, uid string) (map[string]int64, error) {
	conn := d.Get()
	defer conn.Close()

	return stringInt64Map(conn.Do("HGETALL", searchTagvKey(metric, tagK, uid)))
}

func (d *dataAccess) AddMetricTagSet(metric, tagSet, uid string, time int64) error {
	conn := d.Get()
	defer conn.Close()

	_, err := conn.Do("HSET", searchMetricTagSetKey(metric, uid), tagSet, time)
	return slog.Wrap(err)
}

func (d *dataAccess) GetMetricTagSets(metric, uid string, tags opentsdb.TagSet) (map[string]int64, error) {
	conn := d.Get()
	defer conn.Close()

	var cursor = "0"
	result := map[string]int64{}

	for {
		vals, err := redis.Values(conn.Do(d.HSCAN(), searchMetricTagSetKey(metric, uid), cursor))
		if err != nil {
			return nil, slog.Wrap(err)
		}
		cursor, err = redis.String(vals[0], nil)
		if err != nil {
			return nil, slog.Wrap(err)
		}
		mtss, err := stringInt64Map(vals[1], nil)
		if err != nil {
			return nil, slog.Wrap(err)
		}
		for mts, t := range mtss {
			ts, err := opentsdb.ParseTags(mts)
			if err != nil {
				return nil, slog.Wrap(err)
			}
			if ts.Subset(tags) {
				result[mts] = t
			}
		}

		if cursor == "" || cursor == "0" {
			break
		}
	}
	return result, nil
}

func (d *dataAccess) BackupLastInfos(m map[string]map[string]*LastInfo) error {
	conn := d.Get()
	defer conn.Close()

	dat, err := util.MarshalGzipJson(m)
	if err != nil {
		return slog.Wrap(err)
	}
	_, err = conn.Do("SET", "search:last", dat)
	return slog.Wrap(err)
}

func (d *dataAccess) LoadLastInfos() (map[string]map[string]*LastInfo, error) {
	conn := d.Get()
	defer conn.Close()

	b, err := redis.Bytes(conn.Do("GET", "search:last"))
	if err != nil {
		return nil, slog.Wrap(err)
	}
	var m map[string]map[string]*LastInfo
	err = util.UnmarshalGzipJson(b, &m)
	if err != nil {
		return nil, slog.Wrap(err)
	}
	return m, nil
}

//This function not exposed on any public interface. See cmd/bosun/database/test/util/purge_search_data.go for usage.
func (d *dataAccess) PurgeSearchData(metric, uid string, noop bool) error {
	conn := d.Get()
	defer conn.Close()

	tagKeys, err := d.GetTagKeysForMetric(metric, uid)
	if err != nil {
		return err
	}
	fmt.Println("HDEL", searchAllMetricsKey)
	if !noop {
		_, err = conn.Do("HDEL", searchAllMetricsKey(uid), metric)
		if err != nil {
			return err
		}
	}
	hashesToDelete := []string{
		searchMetricTagSetKey(metric, uid),
		searchTagkKey(metric, uid),
	}
	for tagk := range tagKeys {
		hashesToDelete = append(hashesToDelete, searchTagvKey(metric, tagk, uid))
	}
	cmd := d.HCLEAR()
	for _, hash := range hashesToDelete {
		fmt.Println(cmd, hash)
		if !noop {
			_, err = conn.Do(cmd, hash)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *dataAccess) AddHostTagSet(host, uid string, tagSet []opentsdb.TagSet) error {
	conn := d.Get()
	defer conn.Close()
	key := searchHostTagSetKey(host, uid)
	var args []interface {
	}
	now := time.Now().Unix()
	args = append(args, key)
	for _, data := range tagSet {
		for key, value := range data {

			args = append(args, key, value)
			oldValue, err2 := redis.String(conn.Do("HGET", searchHostTagSetKey(host, uid),key))

			if err2 != nil {
				fmt.Println(err2)
			} else {
				if oldValue != value  {
					keys, err3 := stringInt64Map(conn.Do("HGETALL", searchTagSetHostKey(key, oldValue, uid)))

					if err3 != nil {
						fmt.Println(err3)
					} else {
						delete(keys,host)
						if len(keys) == 0 {
							_, err := conn.Do("DEL", searchTagSetHostKey(key, oldValue, uid))
							if err != nil {
								fmt.Println(err)
							}
						}
					}
				}


			}

			_, err := conn.Do("HSET", searchTagSetHostKey(key, value, uid), host, now)
			if err != nil {
				fmt.Println(err)
			}



		}
	}

	_, err := conn.Do("HMSET", args...)
	if err != nil {
		fmt.Println(err)
	}

	return slog.Wrap(err)
}

func (d *dataAccess) DelHostTagSet(host, uid string, tagSet []opentsdb.TagSet) error {
	conn := d.Get()
	defer conn.Close()

	for _, data := range tagSet {
		for key, value := range data {
			_, err := conn.Do("HDEL", searchHostTagSetKey(host, uid), key)
			if err != nil {
				fmt.Println(err)
			}
			_, err = conn.Do("HDEL", searchTagSetHostKey(key, value, uid), host)
			if err != nil {
				fmt.Println(err)
			}

			keys, err2 := stringInt64Map(conn.Do("HGETALL", searchTagSetHostKey(key, value, uid)))

			if err2 != nil {
				fmt.Println(err2)
			} else {
				if len(keys) == 0 {
					_, err = conn.Do("DEL", searchTagSetHostKey(key, value, uid))
					if err != nil {
						fmt.Println(err)
					}
				}
			}
		}
	}

	return nil
}
