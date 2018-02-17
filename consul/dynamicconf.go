package consul

import (
	"context"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/errwrap"
	"github.com/inconshreveable/log15"
)

// TODO: rewrite with functional options
func WatchTree(ctx context.Context, clt *api.Client, pre string, c chan map[string]string, logger log15.Logger) (res map[string]string, err error) {
	// it is our job to close the c channel when we won't write anymore to it
	if clt == nil || len(pre) == 0 {
		logger.Info("Not watching Consul for dynamic configuration")
		sclose(c)
		return nil, nil
	}
	logger.Debug("Getting configuration from Consul", "prefix", pre)

	var idx uint64
	res, idx, err = getTree(ctx, clt, pre, 0)

	if err != nil {
		sclose(c)
		return nil, err
	}

	if c == nil {
		return res, nil
	}

	firstResults := cloneResults(res)

	go func() {
		defer close(c)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if watch(ctx, clt, pre, &idx, &res) {
					c <- res
				}
			}
		}
	}()

	return firstResults, nil

}

func watch(ctx context.Context, client *api.Client, prefix string, idx *uint64, results *map[string]string) bool {
	nextResults, nextIdx, err := getTree(ctx, client, prefix, *idx)

	select {
	case <-ctx.Done():
		return false
	default:
	}

	if err != nil {
		time.Sleep(time.Second)
		return false
	}

	if nextIdx == *idx {
		return false
	}

	if equalResults(nextResults, *results) {
		return false
	}

	*idx = nextIdx
	*results = cloneResults(nextResults)
	return true
}

func getTree(ctx context.Context, clt *api.Client, pre string, idx uint64) (map[string]string, uint64, error) {
	q := &api.QueryOptions{RequireConsistent: true, WaitIndex: idx, WaitTime: 10 * time.Second}
	q = q.WithContext(ctx)
	kvpairs, meta, err := clt.KV().List(pre, q)
	if err != nil {
		return nil, 0, errwrap.Wrapf("Error reading configuration in Consul: {{err}}", err)
	}
	if len(kvpairs) == 0 {
		return nil, meta.LastIndex, nil
	}
	res := map[string]string{}
	for _, v := range kvpairs {
		res[strings.TrimSpace(string(v.Key))] = strings.TrimSpace(string(v.Value))
	}
	return res, meta.LastIndex, nil
}
