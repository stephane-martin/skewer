package consul

import (
	"context"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/errwrap"
	"github.com/inconshreveable/log15"
)

func WatchTree(ctx context.Context, client *api.Client, prefix string, resultsChan chan map[string]string, logger log15.Logger) (results map[string]string, err error) {
	// it is our job to close the notifications channel when we won't write anymore to it
	if client == nil || len(prefix) == 0 {
		logger.Info("Not watching Consul for dynamic configuration")
		sclose(resultsChan)
		return nil, nil
	}
	logger.Debug("Getting configuration from Consul", "prefix", prefix)

	var first_index uint64
	results, first_index, err = getTree(client, prefix, 0)

	if err != nil {
		sclose(resultsChan)
		return nil, err
	}

	if resultsChan == nil {
		return results, nil
	}

	previous_index := first_index
	previous_keyvalues := copy_map(results)

	watch := func() {
		results, index, err := getTree(client, prefix, previous_index)
		if err != nil {
			logger.Warn("Error reading configuration in Consul", "error", err)
			time.Sleep(time.Second)
			return
		}

		is_equal := true

		if index == previous_index {
			return
		}

		if is_equal && len(results) != len(previous_keyvalues) {
			is_equal = false
		}

		if is_equal {
			for k, v := range results {
				last_v, present := previous_keyvalues[k]
				if !present {
					is_equal = false
					break
				}
				if v != last_v {
					is_equal = false
					break
				}
			}
		}

		if !is_equal {
			resultsChan <- results
			previous_index = index
			previous_keyvalues = results
		}
	}

	go func() {
		defer close(resultsChan)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				watch()
			}
		}
	}()

	return results, nil

}

func getTree(client *api.Client, prefix string, waitIndex uint64) (map[string]string, uint64, error) {
	q := &api.QueryOptions{RequireConsistent: true, WaitIndex: waitIndex, WaitTime: 2 * time.Second}
	kvpairs, meta, err := client.KV().List(prefix, q)
	if err != nil {
		return nil, 0, errwrap.Wrapf("Error reading configuration in Consul: {{err}}", err)
	}
	if len(kvpairs) == 0 {
		return nil, meta.LastIndex, nil
	}
	results := map[string]string{}
	for _, v := range kvpairs {
		results[strings.TrimSpace(string(v.Key))] = strings.TrimSpace(string(v.Value))
	}
	return results, meta.LastIndex, nil
}
