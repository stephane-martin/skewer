package consul

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
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

type Service struct {
	ID       string
	Name     string
	IP       string
	parsedIP net.IP
	Port     int
	CheckURL string
	Tags     []string
}

func NewService(name string, ip string, port int, checkURL string, tags []string) *Service {
	s := Service{
		Name: name,
		IP:   ip,
		Port: port,
	}
	checkURL = strings.TrimSpace(checkURL)
	if len(checkURL) == 0 {
		s.CheckURL = fmt.Sprintf("http://[%s]:%d/health", ip, port)
	} else {
		s.CheckURL = checkURL
	}
	if tags == nil {
		s.Tags = []string{}
	} else {
		s.Tags = tags
	}

	localIP, err := LocalIP()
	if err != nil {
		// todo
		localIP = net.ParseIP("127.0.0.1")
	}

	var parsedIP net.IP

	ip = strings.TrimSpace(ip)
	if len(ip) == 0 {
		parsedIP = localIP
	} else {
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			parsedIP = localIP
		}
	}
	if parsedIP.IsUnspecified() { // 0.0.0.0
		parsedIP = localIP
	}
	s.parsedIP = parsedIP

	hostname, err := os.Hostname()
	if err != nil {
		// todo
		hostname = parsedIP.String()
	}

	s.ID = fmt.Sprintf("%s-%s-%s-%d", name, hostname, parsedIP.String(), port)
	return &s
}

type Registry struct {
	client                *api.Client
	logger                log15.Logger
	registeredServicesIds map[string]bool
	RegisterChan          chan *Service
	UnregisterChan        chan *Service
	wgroup                *sync.WaitGroup
}

func (r *Registry) Wait() {
	r.wgroup.Wait()
}

func NewRegistry(ctx context.Context, params ConnParams, logger log15.Logger) (*Registry, error) {
	c, err := NewClient(params)
	if err != nil {
		return nil, err
	}
	r := Registry{client: c, logger: logger}
	r.wgroup = &sync.WaitGroup{}
	r.registeredServicesIds = map[string]bool{}
	r.RegisterChan = make(chan *Service)
	r.UnregisterChan = make(chan *Service)

	r.wgroup.Add(1)
	go func() {
		defer r.wgroup.Done()
		for {
			select {
			case <-ctx.Done():
				for svcID, registered := range r.registeredServicesIds {
					if registered {
						err := doUnregister(r.client, svcID)
						if err == nil {
							r.registeredServicesIds[svcID] = false
						}
					}
				}
				return
			case svc := <-r.RegisterChan:
				if r.registeredServicesIds[svc.ID] {
					// todo: already registered
				} else {
					err := doRegister(r.client, svc)
					if err == nil {
						r.registeredServicesIds[svc.ID] = true
					} else {
						// todo
					}
				}
			case svc := <-r.UnregisterChan:
				if r.registeredServicesIds[svc.ID] {
					err := doUnregister(r.client, svc.ID)
					if err == nil {
						r.registeredServicesIds[svc.ID] = false
					} else {
						// todo
					}
				} else {
					// todo: not registered
				}
			}
		}
	}()

	return &r, nil
}

func doRegister(client *api.Client, svc *Service) error {

	service := &api.AgentServiceRegistration{
		ID:      svc.ID,
		Name:    svc.Name,
		Address: svc.parsedIP.String(),
		Port:    svc.Port,
		Tags:    svc.Tags,
		Check: &api.AgentServiceCheck{
			HTTP:          svc.CheckURL,
			Interval:      "30s",
			Timeout:       "2s",
			TLSSkipVerify: true,
			Status:        "passing",
		},
	}

	return client.Agent().ServiceRegister(service)
	//r.logger.Info("Registered service in Consul", "id", service.ID, "ip", service.Address, "port", service.Port,"tags", strings.Join(service.Tags, ","), "health_url", service.Check.HTTP)
}

func (r *Registry) Registered(serviceID string) (bool, error) {
	services, err := r.client.Agent().Services()
	if err != nil {
		return false, err
	}
	return services[serviceID] != nil, nil
}

func doUnregister(client *api.Client, serviceID string) error {
	return client.Agent().ServiceDeregister(serviceID)
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
