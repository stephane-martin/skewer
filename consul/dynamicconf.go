package consul

import (
	"context"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/errwrap"
	"github.com/inconshreveable/log15"
)

type WatchOpts struct {
	ctx         context.Context
	client      *api.Client
	prefix      string
	resultsChan chan map[string]string
	logger      log15.Logger
	recursive   bool
}

type WatchOpt func(opts *WatchOpts)

func Context(ctx context.Context) WatchOpt {
	return func(opts *WatchOpts) {
		opts.ctx = ctx
	}
}

func Client(client *api.Client) WatchOpt {
	return func(opts *WatchOpts) {
		opts.client = client
	}
}

func Prefix(prefix string) WatchOpt {
	return func(opts *WatchOpts) {
		opts.prefix = prefix
	}
}

func ResultsChan(resultsChan chan map[string]string) WatchOpt {
	return func(opts *WatchOpts) {
		opts.resultsChan = resultsChan
	}
}

func Logger(logger log15.Logger) WatchOpt {
	return func(opts *WatchOpts) {
		opts.logger = logger
	}
}

func Recursive(recursive bool) WatchOpt {
	return func(opts *WatchOpts) {
		opts.recursive = recursive
	}
}

func Watch(opts ...WatchOpt) (res map[string]string, err error) {
	env := WatchOpts{}
	for _, opt := range opts {
		opt(&env)
	}

	// it is our job to close the c channel when we won't write anymore to it
	if env.client == nil || len(env.prefix) == 0 {
		env.logger.Info("Not watching Consul for dynamic configuration")
		sclose(env.resultsChan)
		return nil, nil
	}
	env.logger.Debug("Getting configuration from Consul", "prefix", env.prefix)

	var idx uint64
	res, idx, err = getTree(env.ctx, env.client, env.prefix, 0, env.recursive)

	if err != nil {
		sclose(env.resultsChan)
		return nil, err
	}

	if env.resultsChan == nil {
		return res, nil
	}

	firstResults := cloneResults(res)

	go func() {
		defer close(env.resultsChan)
		for {
			select {
			case <-env.ctx.Done():
				return
			default:
				if watch(env.ctx, env.client, env.prefix, env.recursive, &idx, &res, env.logger) {
					env.resultsChan <- res
				}
			}
		}
	}()

	return firstResults, nil

}

func watch(ctx context.Context, clt *api.Client, pre string, rec bool, idx *uint64, res *map[string]string, l log15.Logger) bool {
	nextResults, nextIdx, err := getTree(ctx, clt, pre, *idx, rec)

	if err != nil {
		l.Warn("Error watching configuration in Consul", "error", err)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(time.Second):
			return false
		}
	}

	select {
	case <-ctx.Done():
		return false
	default:
	}

	if nextIdx == *idx {
		return false
	}

	if equalResults(nextResults, *res) {
		return false
	}

	*idx = nextIdx
	*res = cloneResults(nextResults)
	return true
}

func getTree(ctx context.Context, clt *api.Client, pre string, idx uint64, rec bool) (res map[string]string, m uint64, err error) {
	q := &api.QueryOptions{RequireConsistent: true, WaitIndex: idx, WaitTime: 10 * time.Second}
	q = q.WithContext(ctx)
	var kvpairs api.KVPairs
	var meta *api.QueryMeta

	if rec {
		kvpairs, meta, err = clt.KV().List(pre, q)
	} else {
		var kvpair *api.KVPair
		kvpair, meta, err = clt.KV().Get(pre, q)
		kvpairs = []*api.KVPair{kvpair}
	}
	if err != nil {
		return nil, 0, errwrap.Wrapf("Error reading configuration in Consul: {{err}}", err)
	}
	if len(kvpairs) == 0 {
		return nil, meta.LastIndex, nil
	}
	res = make(map[string]string, len(kvpairs))
	for _, v := range kvpairs {
		res[strings.TrimSpace(string(v.Key))] = strings.TrimSpace(string(v.Value))
	}
	return res, meta.LastIndex, nil
}
