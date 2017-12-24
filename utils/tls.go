package utils

import (
	"crypto/tls"
	"net"
	"path/filepath"
	"strings"

	rootcerts "github.com/hashicorp/go-rootcerts"
)

// NewTLSConfig builds and returns a TLS config from the provided parameters.
func NewTLSConfig(address, caFile, caPath, certFile, keyFile string, insecure bool, confined bool) (*tls.Config, error) {
	tlsClientConfig := &tls.Config{
		InsecureSkipVerify: insecure,
	}

	if address != "" {
		server := address
		hasPort := strings.LastIndex(server, ":") > strings.LastIndex(server, "]")
		if hasPort {
			var err error
			server, _, err = net.SplitHostPort(server)
			if err != nil {
				return nil, err
			}
		}
		tlsClientConfig.ServerName = server
	}

	if len(certFile) > 0 && len(keyFile) > 0 {
		if confined {
			certFile = filepath.Join("/tmp", "certfiles", certFile)
			keyFile = filepath.Join("/tmp", "certfiles", keyFile)
		}
		tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		tlsClientConfig.Certificates = []tls.Certificate{tlsCert}
	}

	if len(caFile) > 0 && confined {
		caFile = filepath.Join("/tmp", "certfiles", caFile)
	}

	if len(caPath) > 0 && confined {
		caPath = filepath.Join("/tmp", "certpaths", caPath)
	}

	rootConfig := &rootcerts.Config{
		CAFile: caFile,
		CAPath: caPath,
	}
	if err := rootcerts.ConfigureTLS(tlsClientConfig, rootConfig); err != nil {
		return nil, err
	}

	return tlsClientConfig, nil
}
