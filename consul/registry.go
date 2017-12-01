package consul

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/errwrap"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/model"
)

type ServiceActionType bool

const (
	REGISTER   ServiceActionType = false
	UNREGISTER                   = true
)

type ServiceAction struct {
	Action  ServiceActionType
	Service *Service
}

type Service struct {
	ID       string
	IP       string
	parsedIP net.IP
	Port     int
	Check    string
	Tags     []string
}

func NewService(ip string, port int, check string, tags []string) (*Service, error) {
	s := Service{
		IP:    ip,
		Port:  port,
		Check: check,
	}

	if tags == nil {
		s.Tags = []string{}
	} else {
		s.Tags = tags
	}

	localIP, err := LocalIP()
	if err != nil {
		return nil, errwrap.Wrapf("Error when trying to get a local IP: {{err}}", err)
	}

	var parsedIP net.IP

	ip = strings.TrimSpace(ip)
	if len(ip) == 0 {
		parsedIP = localIP
	} else {
		parsedIP = net.ParseIP(ip)
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
		return nil, errwrap.Wrapf("Error trying to get the hostname: {{err}}", err)
	}

	s.ID = fmt.Sprintf("skewer-%s-%s-%d", hostname, s.parsedIP.String(), port)
	return &s, nil
}

type Registry struct {
	client                *api.Client
	logger                log15.Logger
	registeredServicesIds map[string]bool
	RegisterChan          chan ServiceAction
	wgroup                *sync.WaitGroup
	svcName               string
}

func (r *Registry) WaitFinished() {
	r.wgroup.Wait()
}

func (r *Registry) RegisterTcpListeners(infos []model.ListenerInfo) {
	for _, info := range infos {
		r.RegisterTcpListener(info)
	}
}

func (r *Registry) RegisterTcpListener(infos model.ListenerInfo) {
	if infos.BindAddr == "" || infos.Port == 0 || infos.Protocol == "" {
		return
	}
	svc, err := NewService(infos.BindAddr, infos.Port, fmt.Sprintf("%s:%d", infos.BindAddr, infos.Port), []string{infos.Protocol})
	if err == nil {
		action := ServiceAction{Action: REGISTER, Service: svc}
		r.RegisterChan <- action
	}
}

func (r *Registry) UnregisterTcpListeners(infos []model.ListenerInfo) {
	for _, info := range infos {
		r.UnregisterTcpListener(info)
	}
}

func (r *Registry) UnregisterTcpListener(infos model.ListenerInfo) {
	if infos.BindAddr == "" || infos.Port == 0 || infos.Protocol == "" {
		return
	}
	svc, err := NewService(infos.BindAddr, infos.Port, fmt.Sprintf("%s:%d", infos.BindAddr, infos.Port), []string{infos.Protocol})
	if err == nil {
		action := ServiceAction{Action: UNREGISTER, Service: svc}
		r.RegisterChan <- action
	}
}

func NewRegistry(ctx context.Context, params ConnParams, svcName string, logger log15.Logger) (*Registry, error) {
	addr := strings.TrimSpace(params.Address)
	if len(addr) == 0 {
		return nil, nil
	}
	c, err := NewClient(params)
	if err != nil {
		return nil, err
	}
	r := Registry{client: c, logger: logger, svcName: strings.TrimSpace(svcName)}
	r.wgroup = &sync.WaitGroup{}
	r.registeredServicesIds = map[string]bool{}
	r.RegisterChan = make(chan ServiceAction)

	r.wgroup.Add(1)
	go func() {
		defer r.wgroup.Done()
		for {
			select {
			case <-ctx.Done():
				for svcID, registered := range r.registeredServicesIds {
					if registered {
						if doUnregister(r.client, svcID) == nil {
							r.registeredServicesIds[svcID] = false
						}
					}
				}
				return
			case serviceAction := <-r.RegisterChan:
				svc := serviceAction.Service
				if !svc.parsedIP.IsLoopback() {
					if serviceAction.Action == REGISTER {
						if r.registeredServicesIds[svc.ID] {
							logger.Info("Service already registed in Consul", "ID", svc.ID)
						} else {
							err := doRegister(r.client, svc, r.svcName)
							if err == nil {
								logger.Debug("Registered in consul", "ID", svc.ID, "IP", svc.IP, "port", svc.Port)
								r.registeredServicesIds[svc.ID] = true
							} else {
								logger.Warn("Failed to register service in Consul", "ID", svc.ID, "IP", svc.IP, "port", svc.Port, "error", err)
							}
						}
					} else {
						if r.registeredServicesIds[svc.ID] {
							err := doUnregister(r.client, svc.ID)
							if err == nil {
								r.registeredServicesIds[svc.ID] = false
								logger.Debug("Unregistered from consul", "ID", svc.ID)
							} else {
								logger.Warn("Failed to unregister service from Consul", "ID", svc.ID, "error", err)
							}
						} else {
							logger.Info("Service is not registered in Consul, and can not be unregistered", "ID", svc.ID)
						}
					}
				}
			}
		}
	}()

	return &r, nil
}

func doRegister(client *api.Client, svc *Service, svcName string) error {

	service := &api.AgentServiceRegistration{
		ID:      svc.ID,
		Name:    svcName,
		Address: svc.parsedIP.String(),
		Port:    svc.Port,
		Tags:    svc.Tags,
	}

	check := strings.TrimSpace(svc.Check)
	if strings.HasPrefix(check, "http://") || strings.HasPrefix(check, "https://") {
		service.Check = &api.AgentServiceCheck{
			HTTP:          svc.Check,
			Interval:      "30s",
			Timeout:       "2s",
			TLSSkipVerify: true,
			Status:        "passing",
		}
	} else if len(check) > 0 {
		service.Check = &api.AgentServiceCheck{
			TCP:      svc.Check,
			Interval: "30s",
			Timeout:  "2s",
			Status:   "passing",
		}
	}

	return client.Agent().ServiceRegister(service)
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
