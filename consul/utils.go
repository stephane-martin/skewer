package consul

import (
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/errwrap"
)

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

type ConnParams struct {
	Address    string
	Datacenter string
	Token      string
	CAFile     string
	CAPath     string
	CertFile   string
	KeyFile    string
	Insecure   bool
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
		config.TLSConfig = api.TLSConfig{
			Address:            addr,
			CAFile:             params.CAFile,
			CAPath:             params.CAPath,
			CertFile:           params.CertFile,
			KeyFile:            params.KeyFile,
			InsecureSkipVerify: params.Insecure,
		}
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
