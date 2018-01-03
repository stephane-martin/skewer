package conf

import (
	"fmt"
	"strings"

	"github.com/stephane-martin/skewer/model/encoders"
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
	UDP                     = 2
	TCP                     = 4
	RELP                    = 8
	File                    = 16
	Stderr                  = 32
	Graylog                 = 64
	HTTP                    = 128
	NATS                    = 256
)

var Destinations = map[string]DestinationType{
	"kafka":   Kafka,
	"udp":     UDP,
	"tcp":     TCP,
	"relp":    RELP,
	"file":    File,
	"stderr":  Stderr,
	"graylog": Graylog,
	"http":    HTTP,
	"nats":    NATS,
}

var DestinationNames = map[DestinationType]string{
	Kafka:   "kafka",
	UDP:     "udp",
	TCP:     "tcp",
	RELP:    "relp",
	File:    "file",
	Stderr:  "stderr",
	Graylog: "graylog",
	HTTP:    "http",
	NATS:    "nats",
}

var RDestinations = map[DestinationType]byte{
	Kafka:   'k',
	UDP:     'u',
	TCP:     't',
	RELP:    'r',
	File:    'f',
	Stderr:  's',
	Graylog: 'g',
	HTTP:    'h',
	NATS:    'n',
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
	c.UDPDest.Format = strings.TrimSpace(strings.ToLower(c.UDPDest.Format))
	c.TCPDest.Format = strings.TrimSpace(strings.ToLower(c.TCPDest.Format))
	c.HTTPDest.Format = strings.TrimSpace(strings.ToLower(c.HTTPDest.Format))
	c.RELPDest.Format = strings.TrimSpace(strings.ToLower(c.RELPDest.Format))
	c.KafkaDest.Format = strings.TrimSpace(strings.ToLower(c.KafkaDest.Format))
	c.FileDest.Format = strings.TrimSpace(strings.ToLower(c.FileDest.Format))
	c.StderrDest.Format = strings.TrimSpace(strings.ToLower(c.StderrDest.Format))

	for _, frmt := range []string{
		c.UDPDest.Format,
		c.TCPDest.Format,
		c.HTTPDest.Format,
		c.RELPDest.Format,
		c.KafkaDest.Format,
		c.FileDest.Format,
		c.StderrDest.Format,
	} {
		if encoders.ParseFormat(frmt) == -1 {
			return ConfigurationCheckError{ErrString: fmt.Sprintf("Unknown destination format: '%s'", frmt)}
		}
	}
	return nil
}
