package consul

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/hashicorp/consul/api"
	"github.com/inconshreveable/log15"
)

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
