package conf

import (
	"fmt"
	"strings"

	"github.com/stephane-martin/skewer/model"
)

// DestinationType lists the possible kind of destinations where skewer can forward messages.
type DestinationType uint64

func (dests DestinationType) Iterate() (res []DestinationType) {
	if dests == 0 {
		return []DestinationType{Stderr}
	}
	res = make([]DestinationType, 0, len(Destinations))
	for _, dtype := range Destinations {
		if uint64(dests)&uint64(dtype) != 0 {
			res = append(res, dtype)
		}
	}
	return res
}

const (
	Kafka   DestinationType = 1
	Udp                     = 2
	Tcp                     = 4
	Relp                    = 8
	File                    = 16
	Stderr                  = 32
	Graylog                 = 64
	Http                    = 128
)

var Destinations = map[string]DestinationType{
	"kafka":   Kafka,
	"udp":     Udp,
	"tcp":     Tcp,
	"relp":    Relp,
	"file":    File,
	"stderr":  Stderr,
	"graylog": Graylog,
	"http":    Http,
}

var DestinationNames = map[DestinationType]string{
	Kafka:   "kafka",
	Udp:     "udp",
	Tcp:     "tcp",
	Relp:    "relp",
	File:    "file",
	Stderr:  "stderr",
	Graylog: "graylog",
	Http:    "http",
}

var RDestinations = map[DestinationType]byte{
	Kafka:   'k',
	Udp:     'u',
	Tcp:     't',
	Relp:    'r',
	File:    'f',
	Stderr:  's',
	Graylog: 'g',
	Http:    'h',
}

func (m *MainConfig) GetDestinations() (dests DestinationType, err error) {
	destr := strings.TrimSpace(strings.ToLower(m.Destination))
	for _, dest := range strings.Split(destr, ",") {
		d, ok := Destinations[strings.TrimSpace(dest)]
		if ok {
			dests = dests | d
		} else {
			return 0, ConfigurationCheckError{ErrString: fmt.Sprintf("Unknown destination type: '%s'", dest)}
		}
	}
	if dests == 0 {
		return Stderr, nil
	}
	return
}

func (c *BaseConfig) CheckDestinations() error {
	// note that Graylog destination does not have a Format option
	c.UdpDest.Format = strings.TrimSpace(strings.ToLower(c.UdpDest.Format))
	c.TcpDest.Format = strings.TrimSpace(strings.ToLower(c.TcpDest.Format))
	c.HTTPDest.Format = strings.TrimSpace(strings.ToLower(c.HTTPDest.Format))
	c.RelpDest.Format = strings.TrimSpace(strings.ToLower(c.RelpDest.Format))
	c.KafkaDest.Format = strings.TrimSpace(strings.ToLower(c.KafkaDest.Format))
	c.FileDest.Format = strings.TrimSpace(strings.ToLower(c.FileDest.Format))
	c.StderrDest.Format = strings.TrimSpace(strings.ToLower(c.StderrDest.Format))

	for _, frmt := range []string{
		c.UdpDest.Format,
		c.TcpDest.Format,
		c.HTTPDest.Format,
		c.RelpDest.Format,
		c.KafkaDest.Format,
		c.FileDest.Format,
		c.StderrDest.Format,
	} {
		_, err := model.NewEncoder(frmt)
		if err != nil {
			return ConfigurationCheckError{ErrString: fmt.Sprintf("Unknown destination format: '%s'", frmt)}
		}
	}
	return nil
}
