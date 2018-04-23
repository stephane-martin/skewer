package consul

import (
	"net"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

func LocalIP() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, eerrors.Wrap(err, "Error retrieving interfaces")
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
	Address    string `json:"address"`
	Datacenter string `json:"datacenter"`
	Token      string `json:"token"`
	CAFile     string `json:"ca_file"`
	CAPath     string `json:"ca_path"`
	CertFile   string `json:"cert_file"`
	KeyFile    string `json:"key_file"`
	Insecure   bool   `json:"insecure"`
	Key        string `json:"key"`
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
		return nil, eerrors.New("consul addr must start with 'http://' or 'https://'")
	}
	config.Address = addr
	config.Token = strings.TrimSpace(params.Token)
	config.Datacenter = strings.TrimSpace(params.Datacenter)

	client, err := api.NewClient(&config)
	if err != nil {
		return nil, eerrors.Wrap(err, "Error creating the consul client")
	}
	return client, nil
}

func sclose(c chan map[string]string) {
	if c != nil {
		close(c)
	}
}

func equalResults(this, that map[string]string) bool {
	if this == nil || that == nil {
		return this == nil && that == nil
	}
	if len(this) != len(that) {
		return false
	}
	for k, v := range this {
		thatv, ok := that[k]
		if !ok {
			return false
		}
		if !(v == thatv) {
			return false
		}
	}
	return true
}

func cloneResults(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	return deepCopyMap(src)
}

func deepCopyMap(src map[string]string) (dst map[string]string) {
	dst = make(map[string]string)
	for srcKey, srcValue := range src {
		dst[srcKey] = srcValue
	}
	return dst
}
