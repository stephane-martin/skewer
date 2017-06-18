package consul

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/errwrap"
	"github.com/inconshreveable/log15"
)

func copy_map(m map[string]string) map[string]string {
	c := map[string]string{}
	for k, v := range m {
		c[k] = v
	}
	return c
}

func sclose(c chan map[string]string) {
	if c != nil {
		close(c)
	}
}

type ConnParams struct {
	Address    string
	Datacenter string
	Token      string
}

func NewClient(params ConnParams) (*api.Client, error) {

	config := *api.DefaultConfig()
	addr := strings.TrimSpace(params.Address)
	if strings.HasPrefix(addr, "http://") {
		config.Scheme = "http"
		addr = addr[7:]
	} else if strings.HasPrefix(addr, "https://") {
		config.Scheme = "https"
		addr = addr[8:]
	} else {
		return nil, fmt.Errorf("consul addr must start with 'http://' or 'https://'")
	}
	config.Address = addr
	config.Token = strings.TrimSpace(params.Token)
	config.Datacenter = strings.TrimSpace(params.Datacenter)

	client, err := api.NewClient(&config)
	if err != nil {
		return nil, errwrap.Wrapf("Error creating the consul client: {{err}}", err)
	}
	return client, nil
}

func WatchTree(client *api.Client, prefix string, resultsChan chan map[string]string, logger log15.Logger) (results map[string]string, stop chan bool, err error) {
	// it is our job to close the notifications channel when we won't write anymore to it
	if client == nil || len(prefix) == 0 {
		logger.Info("Not watching Consul for dynamic configuration")
		sclose(resultsChan)
		return nil, nil, nil
	}
	logger.Debug("Getting configuration from Consul", "prefix", prefix)

	var first_index uint64
	results, first_index, err = getTree(client, prefix, 0)

	if err != nil {
		sclose(resultsChan)
		return nil, nil, err
	}

	if resultsChan == nil {
		return results, nil, nil
	}

	stop = make(chan bool, 1)
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
			case <-stop:
				return
			default:
				watch()
			}
		}
	}()

	return results, stop, nil

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

type Registry struct {
	client *api.Client
	logger log15.Logger
}

func NewRegistry(params ConnParams, logger log15.Logger) (*Registry, error) {
	c, err := NewClient(params)
	if err != nil {
		return nil, err
	}
	r := Registry{client: c, logger: logger}
	return &r, nil
}

func (r *Registry) Register(name string, ip_s string, port int, check_url string, tags []string) (service_id string, err error) {

	ip := net.ParseIP(ip_s)
	if ip == nil {
		ip, err = LocalIP()
		if err != nil {
			return "", err
		}
	}
	if ip.IsLoopback() {
		r.logger.Info("Skipping registration of service: it listens on loopback", "name", name)
		return "", nil
	}
	if ip.IsUnspecified() { // 0.0.0.0
		ip, err = LocalIP()
		if err != nil {
			return "", err
		}
	}
	var hostname string
	hostname, err = os.Hostname()
	if err != nil {
		return "", err
	}

	service_id = fmt.Sprintf("%s-%s-%d", name, hostname, port)

	if len(check_url) == 0 {
		check_url = fmt.Sprintf("http://[%s]:%d/health", ip, port)
	}

	service := &api.AgentServiceRegistration{
		ID:      service_id,
		Name:    name,
		Address: ip.String(),
		Port:    port,
		Tags:    tags,
		Check: &api.AgentServiceCheck{
			HTTP:          check_url,
			Interval:      "30s",
			Timeout:       "2s",
			TLSSkipVerify: true,
			Status:        "passing",
		},
	}

	err = r.client.Agent().ServiceRegister(service)
	if err != nil {
		return "", err
	}
	r.logger.Info("Registered service in Consul", "id", service.ID, "ip", service.Address, "port", service.Port,
		"tags", strings.Join(service.Tags, ","), "health_url", service.Check.HTTP)
	return service_id, nil
}

func (r *Registry) Registered(service_id string) (bool, error) {
	services, err := r.client.Agent().Services()
	if err != nil {
		return false, err
	}
	return services[service_id] != nil, nil
}

func (r *Registry) Unregister(service_id string, logger log15.Logger) error {
	err := r.client.Agent().ServiceDeregister(service_id)
	if err != nil {
		logger.Error("Failed to unregister service", "id", service_id)
		return err
	}
	logger.Info("Unregistered service from Consul", "id", service_id)
	return nil
}

func LocalIP() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.IsGlobalUnicast() {
			if ipnet.IP.To4() != nil || ipnet.IP.To16() != nil {
				return ipnet.IP, nil
			}
		}
	}
	return nil, nil
}
