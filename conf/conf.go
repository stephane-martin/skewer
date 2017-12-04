package conf

//go:generate goderive .

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	sarama "gopkg.in/Shopify/sarama.v1"

	"github.com/BurntSushi/toml"
	"github.com/hashicorp/errwrap"
	"github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/utils"
)

func (source BaseConfig) Clone() BaseConfig {
	//return source
	return deriveCloneBaseConfig(source)
}

func newBaseConf() BaseConfig {
	brokers := []string{}
	baseConf := BaseConfig{
		TcpSource:  []TcpSourceConfig{},
		UdpSource:  []UdpSourceConfig{},
		RelpSource: []RelpSourceConfig{},
		KafkaDest:  KafkaDestConfig{Brokers: brokers},
		Store:      StoreConfig{},
		Parsers:    []ParserConfig{},
		Journald:   JournaldConfig{},
		Metrics:    MetricsConfig{},
	}
	return baseConf
}

func Default() (BaseConfig, error) {
	v := viper.New()
	SetDefaults(v)
	baseConf := newBaseConf()
	err := v.Unmarshal(&baseConf)
	if err != nil {
		return baseConf, ConfigurationSyntaxError{Err: err}
	}
	err = baseConf.Complete(nil)
	if err != nil {
		return baseConf, ConfigurationSyntaxError{Err: err}
	}
	return baseConf, nil
}

func (c *BaseConfig) String() string {
	return c.Export()
}

type KafkaVersion [4]int

var V0_8_2_0 = KafkaVersion{0, 8, 2, 0}
var V0_8_2_1 = KafkaVersion{0, 8, 2, 1}
var V0_8_2_2 = KafkaVersion{0, 8, 2, 2}
var V0_9_0_0 = KafkaVersion{0, 9, 0, 0}
var V0_9_0_1 = KafkaVersion{0, 9, 0, 1}
var V0_10_0_0 = KafkaVersion{0, 10, 0, 0}
var V0_10_0_1 = KafkaVersion{0, 10, 0, 1}
var V0_10_1_0 = KafkaVersion{0, 10, 1, 0}
var V0_10_2_0 = KafkaVersion{0, 10, 2, 0}

func ParseVersion(v string) (skv sarama.KafkaVersion, e error) {
	var ver KafkaVersion
	for i, n := range strings.SplitN(v, ".", 4) {
		ver[i], e = strconv.Atoi(n)
		if e != nil {
			return skv, ConfigurationCheckError{ErrString: fmt.Sprintf("Kafka Version has invalid format: '%s'", v)}
		}
	}
	return ver.ToSaramaVersion()
}

func (l KafkaVersion) ToSaramaVersion() (v sarama.KafkaVersion, e error) {
	if l.Greater(V0_10_2_0) {
		return sarama.V0_10_2_0, nil
	}
	if l.Greater(V0_10_1_0) {
		return sarama.V0_10_1_0, nil
	}
	if l.Greater(V0_10_0_1) {
		return sarama.V0_10_1_0, nil
	}
	if l.Greater(V0_10_0_0) {
		return sarama.V0_10_0_0, nil
	}
	if l.Greater(V0_9_0_1) {
		return sarama.V0_9_0_1, nil
	}
	if l.Greater(V0_9_0_0) {
		return sarama.V0_9_0_0, nil
	}
	if l.Greater(V0_8_2_2) {
		return sarama.V0_8_2_2, nil
	}
	if l.Greater(V0_8_2_1) {
		return sarama.V0_8_2_1, nil
	}
	if l.Greater(V0_8_2_0) {
		return sarama.V0_8_2_0, nil
	}
	return v, ConfigurationCheckError{ErrString: "Minimal Kafka version is 0.8.2.0"}
}

func (l KafkaVersion) Greater(r KafkaVersion) bool {
	if l[0] > r[0] {
		return true
	}
	if l[0] < r[0] {
		return false
	}
	if l[1] > r[1] {
		return true
	}
	if l[1] < r[1] {
		return false
	}
	if l[2] > r[2] {
		return true
	}
	if l[2] < r[2] {
		return false
	}
	if l[3] >= r[3] {
		return true
	}
	return false
}

func (c *FilterSubConfig) CalculateID() []byte {
	return fnv.New128a().Sum(c.Export())
}

func (c *TcpSourceConfig) SetConfID() {
	copy(c.ConfID[:], c.FilterSubConfig.CalculateID())
}

func (c *UdpSourceConfig) SetConfID() {
	copy(c.ConfID[:], c.FilterSubConfig.CalculateID())
}

func (c *RelpSourceConfig) SetConfID() {
	copy(c.ConfID[:], c.FilterSubConfig.CalculateID())
}

func (c *JournaldConfig) SetConfID() {
	copy(c.ConfID[:], c.FilterSubConfig.CalculateID())
}

func (c *AccountingConfig) SetConfID() {
	copy(c.ConfID[:], c.FilterSubConfig.CalculateID())
}

func (c *TcpSourceConfig) GetClientAuthType() tls.ClientAuthType {
	return convertClientAuthType(c.ClientAuthType)
}

func (c *RelpSourceConfig) GetClientAuthType() tls.ClientAuthType {
	return convertClientAuthType(c.ClientAuthType)
}

func convertClientAuthType(auth_type string) tls.ClientAuthType {
	//s := strings.TrimSpace(c.ClientAuthType)
	s := strings.TrimSpace(auth_type)
	if len(s) == 0 {
		return tls.NoClientCert
	}
	s = strings.ToLower(s)
	s = strings.Replace(s, "_", "", -1)
	switch s {
	case "noclientcert":
		return tls.NoClientCert
	case "requestclientcert":
		return tls.RequestClientCert
	case "requireanyclientcert":
		return tls.RequireAnyClientCert
	case "verifyclientcertifgiven":
		return tls.VerifyClientCertIfGiven
	case "requireandverifyclientcert":
		return tls.RequireAndVerifyClientCert
	default:
		return tls.NoClientCert
	}
}

func (c *SyslogSourceBaseConfig) GetListenAddr() (string, error) {
	if len(c.UnixSocketPath) > 0 {
		return "", nil
	}
	bindIP := net.ParseIP(c.BindAddr)
	if bindIP == nil {
		return "", fmt.Errorf("bind_addr is not an IP address: %s", c.BindAddr)
	}

	if bindIP.IsUnspecified() {
		return fmt.Sprintf(":%d", c.Port), nil
	} else {
		return fmt.Sprintf("%s:%d", bindIP.String(), c.Port), nil
	}
}

func (c *TcpSourceConfig) Export() []byte {
	b, _ := json.Marshal(c)
	return b
}

func (c *UdpSourceConfig) Export() []byte {
	b, _ := json.Marshal(c)
	return b
}

func (c *RelpSourceConfig) Export() []byte {
	b, _ := json.Marshal(c)
	return b
}

func (c *FilterSubConfig) Export() []byte {
	b, _ := json.Marshal(c)
	return b
}

func ImportSyslogConfig(data []byte) (*FilterSubConfig, error) {
	c := FilterSubConfig{}
	err := json.Unmarshal(data, &c)
	if err != nil {
		return nil, fmt.Errorf("Can't unmarshal the syslog config: %s", err.Error())
	}
	return &c, nil
}

func (c *KafkaDestConfig) GetSaramaConfig() (*sarama.Config, error) {
	s := sarama.NewConfig()
	s.ClientID = c.ClientID
	s.ChannelBufferSize = c.ChannelBufferSize
	s.Net.MaxOpenRequests = c.MaxOpenRequests
	s.Net.DialTimeout = c.DialTimeout
	s.Net.ReadTimeout = c.ReadTimeout
	s.Net.WriteTimeout = c.WriteTimeout
	s.Net.KeepAlive = c.KeepAlive
	s.Metadata.Retry.Backoff = c.MetadataRetryBackoff
	s.Metadata.Retry.Max = c.MetadataRetryMax
	s.Metadata.RefreshFrequency = c.MetadataRefreshFrequency
	s.Producer.MaxMessageBytes = c.MessageBytesMax
	s.Producer.RequiredAcks = sarama.RequiredAcks(c.RequiredAcks)
	s.Producer.Timeout = c.ProducerTimeout
	s.Producer.Return.Errors = true
	s.Producer.Return.Successes = true
	s.Producer.Flush.Bytes = c.FlushBytes
	s.Producer.Flush.Frequency = c.FlushFrequency
	s.Producer.Flush.Messages = c.FlushMessages
	s.Producer.Flush.MaxMessages = c.FlushMessagesMax
	s.Producer.Retry.Backoff = c.RetrySendBackoff
	s.Producer.Retry.Max = c.RetrySendMax

	v, _ := ParseVersion(c.Version) // the ignored error has been checked at launch
	s.Version = v

	switch strings.TrimSpace(strings.ToLower(c.Compression)) {
	case "snappy":
		s.Producer.Compression = sarama.CompressionSnappy
	case "gzip":
		s.Producer.Compression = sarama.CompressionGZIP
	case "lz4":
		s.Producer.Compression = sarama.CompressionLZ4
	default:
		s.Producer.Compression = sarama.CompressionNone
	}

	if c.TLSEnabled {
		tlsConf, err := utils.NewTLSConfig("", c.CAFile, c.CAPath, c.CertFile, c.KeyFile, c.Insecure)
		if err == nil {
			s.Net.TLS.Enable = true
			s.Net.TLS.Config = tlsConf
		} else {
			return nil, errwrap.Wrapf("Error building the TLS configuration for Kafka: {{err}}", err)
		}

	}

	switch c.Partitioner {
	case "manual":
		s.Producer.Partitioner = sarama.NewManualPartitioner
	case "random":
		s.Producer.Partitioner = sarama.NewRandomPartitioner
	case "roundrobin":
		s.Producer.Partitioner = sarama.NewRoundRobinPartitioner
	default:
		s.Producer.Partitioner = sarama.NewHashPartitioner
	}

	// MetricRegistry ?
	return s, nil
}

func (c *KafkaDestConfig) GetAsyncProducer() (sarama.AsyncProducer, error) {
	conf, err := c.GetSaramaConfig()
	if err != nil {
		return nil, err
	}
	p, err := sarama.NewAsyncProducer(c.Brokers, conf)
	if err == nil {
		return p, nil
	}
	return nil, KafkaError{Err: err}
}

func (c *KafkaDestConfig) GetClient() (sarama.Client, error) {
	conf, err := c.GetSaramaConfig()
	if err != nil {
		return nil, err
	}
	cl, err := sarama.NewClient(c.Brokers, conf)
	if err == nil {
		return cl, nil
	}
	return nil, KafkaError{Err: err}
}

func InitLoad(ctx context.Context, confDir string, params consul.ConnParams, r kring.Ring, logger log15.Logger) (c BaseConfig, updated chan *BaseConfig, err error) {
	defer func() {
		// sometimes viper panics... let's catch that
		if r := recover(); r != nil {
			// find out exactly what the error was and set err
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}
			logger.Error("Recovered in conf.InitLoad()", "error", err)
		}
		if err != nil {
			c = newBaseConf()
			updated = nil
		}
	}()

	var firstResults map[string]string
	var consulResults chan map[string]string

	v := viper.New()
	SetDefaults(v)
	v.SetConfigName("skewer")

	confDir = strings.TrimSpace(confDir)
	if len(confDir) > 0 {
		v.AddConfigPath(confDir)
	}
	if confDir != "/nonexistent" {
		v.AddConfigPath("/etc")
	}

	err = v.ReadInConfig()
	if err != nil {
		switch err.(type) {
		default:
			return newBaseConf(), nil, ConfigurationReadError{err}
		case viper.ConfigFileNotFoundError:
			logger.Info("No configuration file was found")
		}
	}

	var baseConf BaseConfig
	err = v.Unmarshal(&baseConf)
	if err != nil {
		return newBaseConf(), nil, ConfigurationSyntaxError{Err: err, Filename: v.ConfigFileUsed()}
	}
	c = baseConf.Clone()

	var watchCtx context.Context
	var cancelWatch context.CancelFunc

	consulAddr := strings.TrimSpace(params.Address)
	if len(consulAddr) > 0 {
		clt, err := consul.NewClient(params)
		if err == nil {
			consulResults = make(chan map[string]string, 10)
			watchCtx, cancelWatch = context.WithCancel(ctx)
			firstResults, err = consul.WatchTree(watchCtx, clt, params.Prefix, consulResults, logger)
			if err == nil {
				err = c.ParseParamsFromConsul(firstResults, params.Prefix, logger)
				if err != nil {
					logger.Error("Error decoding configuration from Consul", "error", err)
				}
			} else {
				logger.Error("Error reading from Consul", "error", err)
				cancelWatch()
				consulResults = nil
			}
		} else {
			logger.Error("Error creating Consul client: configuration will not be fetched from Consul", "error", err)
		}
	} else {
		logger.Info("Configuration is not fetched from Consul")
	}

	err = c.Complete(r)
	if err != nil {
		if cancelWatch != nil {
			cancelWatch()
		}
		return newBaseConf(), nil, err
	}

	if consulResults != nil {
		// watch for updates from Consul
		updated = make(chan *BaseConfig)
		go func() {
			for result := range consulResults {
				newConfig := baseConf.Clone()
				err := newConfig.ParseParamsFromConsul(result, params.Prefix, logger)
				if err == nil {
					err = newConfig.Complete(r)
					if err == nil {
						updated <- &newConfig
					} else {
						logger.Error("Error updating conf from Consul", "error", err)
					}
				} else {
					logger.Error("Error decoding conf from Consul", "error", err)
				}
			}
			close(updated)
		}()
	}

	return c, updated, nil
}

func (c *BaseConfig) ParseParamsFromConsul(params map[string]string, prefix string, logger log15.Logger) error {
	rawTcpSourceConf := map[string]map[string]string{}
	rawUdpSourceConf := map[string]map[string]string{}
	rawRelpSourceConf := map[string]map[string]string{}
	rawJournalConf := map[string]string{}
	rawKafkaConf := map[string]string{}
	rawStoreConf := map[string]string{}
	rawMetricsConf := map[string]string{}
	rawParsersConf := map[string]map[string]string{}
	rawAccountingConf := map[string]string{}
	rawMainConf := map[string]string{}
	prefixLen := len(prefix)

	for k, v := range params {
		k = strings.Trim(k[prefixLen:], "/")
		splits := strings.Split(k, "/")
		switch splits[0] {
		case "tcp_source":
			if len(splits) == 3 {
				if _, ok := rawTcpSourceConf[splits[1]]; !ok {
					rawTcpSourceConf[splits[1]] = map[string]string{}
				}
				rawTcpSourceConf[splits[1]][splits[2]] = v
			} else {
				logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		case "udp_source":
			if len(splits) == 3 {
				if _, ok := rawUdpSourceConf[splits[1]]; !ok {
					rawUdpSourceConf[splits[1]] = map[string]string{}
				}
				rawUdpSourceConf[splits[1]][splits[2]] = v
			} else {
				logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		case "relp_source":
			if len(splits) == 3 {
				if _, ok := rawRelpSourceConf[splits[1]]; !ok {
					rawRelpSourceConf[splits[1]] = map[string]string{}
				}
				rawRelpSourceConf[splits[1]][splits[2]] = v
			} else {
				logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		case "journald":
			if len(splits) == 2 {
				rawJournalConf[splits[1]] = v
			} else {
				logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		case "kafka":
			if len(splits) == 2 {
				rawKafkaConf[splits[1]] = v
			} else {
				logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		case "store":
			if len(splits) == 2 {
				rawStoreConf[splits[1]] = v
			} else {
				logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		case "parsers":
			if len(splits) == 3 {
				if _, ok := rawParsersConf[splits[1]]; !ok {
					rawParsersConf[splits[1]] = map[string]string{}
				}
				rawParsersConf[splits[1]][splits[2]] = v
			} else {
				logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		case "metrics":
			if len(splits) == 2 {
				rawMetricsConf[splits[1]] = v
			} else {
				logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		case "accounting":
			if len(splits) == 2 {
				rawAccountingConf[splits[1]] = v
			} else {
				logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		case "main":
			if len(splits) == 2 {
				rawMainConf[splits[1]] = v
			} else {
				logger.Debug("Ignoring Consul KV", "key", k, "value", v)
			}
		default:
			logger.Debug("Ignoring Consul KV", "key", k, "value", v)
		}
	}

	var vi *viper.Viper

	tcpSourceConfs := []TcpSourceConfig{}
	for _, syslogConf := range rawTcpSourceConf {
		vi = viper.New()
		for k, v := range syslogConf {
			vi.Set(k, v)
		}
		sconf := TcpSourceConfig{}
		err := vi.Unmarshal(&sconf)
		if err == nil {
			tcpSourceConfs = append(tcpSourceConfs, sconf)
		} else {
			return err
		}
	}

	udpSourceConfs := []UdpSourceConfig{}
	for _, syslogConf := range rawUdpSourceConf {
		vi = viper.New()
		for k, v := range syslogConf {
			vi.Set(k, v)
		}
		sconf := UdpSourceConfig{}
		err := vi.Unmarshal(&sconf)
		if err == nil {
			udpSourceConfs = append(udpSourceConfs, sconf)
		} else {
			return err
		}
	}

	relpSourceConfs := []RelpSourceConfig{}
	for _, syslogConf := range rawRelpSourceConf {
		vi = viper.New()
		for k, v := range syslogConf {
			vi.Set(k, v)
		}
		sconf := RelpSourceConfig{}
		err := vi.Unmarshal(&sconf)
		if err == nil {
			relpSourceConfs = append(relpSourceConfs, sconf)
		} else {
			return err
		}
	}

	parsersConf := []ParserConfig{}
	for parserName, pConf := range rawParsersConf {
		parserConf := ParserConfig{Name: parserName}
		vi := viper.New()
		for k, v := range pConf {
			vi.Set(k, v)
		}
		err := vi.Unmarshal(&parserConf)
		if err == nil {
			parsersConf = append(parsersConf, parserConf)
		} else {
			return err
		}
	}

	journalConf := JournaldConfig{}
	if len(rawJournalConf) > 0 {
		vi = viper.New()
		SetJournaldDefaults(vi, false)
		for k, v := range rawJournalConf {
			vi.Set(k, v)
		}
		err := vi.Unmarshal(&journalConf)
		if err != nil {
			return err
		}
		c.Journald = journalConf
	}

	kafkaConf := KafkaDestConfig{}
	if len(rawKafkaConf) > 0 {
		vi = viper.New()
		SetKafkaDefaults(vi, false)
		for k, v := range rawKafkaConf {
			vi.Set(k, v)
		}
		err := vi.Unmarshal(&kafkaConf)
		if err != nil {
			return err
		}
		c.KafkaDest = kafkaConf
	}

	storeConf := StoreConfig{}
	if len(rawStoreConf) > 0 {
		vi = viper.New()
		SetStoreDefaults(vi, false)
		for k, v := range rawStoreConf {
			vi.Set(k, v)
		}
		err := vi.Unmarshal(&storeConf)
		if err != nil {
			return err
		}
		c.Store = storeConf
	}

	metricsConf := MetricsConfig{}
	if len(rawMetricsConf) > 0 {
		vi = viper.New()
		SetMetricsDefaults(vi, false)
		for k, v := range rawMetricsConf {
			vi.Set(k, v)
		}
		err := vi.Unmarshal(&metricsConf)
		if err != nil {
			return err
		}
		c.Metrics = metricsConf
	}

	accountingConf := AccountingConfig{}
	if len(rawAccountingConf) > 0 {
		vi = viper.New()
		SetAccountingDefaults(vi, false)
		for k, v := range rawAccountingConf {
			vi.Set(k, v)
		}
		err := vi.Unmarshal(&accountingConf)
		if err != nil {
			return err
		}
		c.Accounting = accountingConf
	}

	mainConf := MainConfig{}
	if len(rawMainConf) > 0 {
		vi = viper.New()
		SetMainDefaults(vi, false)
		for k, v := range rawMainConf {
			vi.Set(k, v)
		}
		err := vi.Unmarshal(&mainConf)
		if err != nil {
			return err
		}
		c.Main = mainConf
	}
	c.TcpSource = append(c.TcpSource, tcpSourceConfs...)
	c.UdpSource = append(c.UdpSource, udpSourceConfs...)
	c.RelpSource = append(c.RelpSource, relpSourceConfs...)
	c.Parsers = append(c.Parsers, parsersConf...)

	return nil
}

func (c *BaseConfig) Export() string {
	buf := new(bytes.Buffer)
	encoder := toml.NewEncoder(buf)
	encoder.Encode(c)
	return buf.String()
}

func (c *BaseConfig) Complete(r kring.Ring) (err error) {
	parsersNames := map[string]bool{}
	for _, parserConf := range c.Parsers {
		name := strings.TrimSpace(parserConf.Name)
		switch name {
		case "rfc5424", "rfc3164", "json", "auto":
			return ConfigurationCheckError{ErrString: "Parser configuration must not use a reserved name"}
		case "":
			return ConfigurationCheckError{ErrString: "Empty parser name"}
		default:
			if _, ok := parsersNames[name]; ok {
				return ConfigurationCheckError{ErrString: "The same parser name is used multiple times"}
			}
			f := strings.TrimSpace(parserConf.Func)
			if len(f) == 0 {
				return ConfigurationCheckError{ErrString: "Empty parser func"}
			}
			parsersNames[name] = true
		}
	}

	_, err = c.Main.GetDestinations()
	if err != nil {
		return err
	}

	c.UdpDest.Format = strings.TrimSpace(strings.ToLower(c.UdpDest.Format))
	c.TcpDest.Format = strings.TrimSpace(strings.ToLower(c.TcpDest.Format))
	c.RelpDest.Format = strings.TrimSpace(strings.ToLower(c.RelpDest.Format))
	c.KafkaDest.Format = strings.TrimSpace(strings.ToLower(c.KafkaDest.Format))
	c.FileDest.Format = strings.TrimSpace(strings.ToLower(c.FileDest.Format))
	c.StderrDest.Format = strings.TrimSpace(strings.ToLower(c.StderrDest.Format))

	for _, frmt := range []string{
		c.UdpDest.Format,
		c.TcpDest.Format,
		c.RelpDest.Format,
		c.KafkaDest.Format,
		c.FileDest.Format,
		c.StderrDest.Format,
	} {
		_, err := model.NewEncoder(os.Stderr, frmt)
		if err != nil {
			return ConfigurationCheckError{ErrString: fmt.Sprintf("Unknown destination format: '%s'", frmt)}
		}
	}

	_, err = ParseVersion(c.KafkaDest.Version)
	if err != nil {
		return ConfigurationCheckError{ErrString: "Kafka version can't be parsed", Err: err}
	}

	syslogConfs := []SyslogSourceConfig{}
	for i := range c.TcpSource {
		syslogConfs = append(syslogConfs, &c.TcpSource[i])
	}
	for i := range c.UdpSource {
		syslogConfs = append(syslogConfs, &c.UdpSource[i])
	}
	for i := range c.RelpSource {
		syslogConfs = append(syslogConfs, &c.RelpSource[i])
	}
	for _, syslogConf := range syslogConfs {
		baseConf := syslogConf.GetSyslogConf()
		filterConf := syslogConf.GetFilterConf()

		if baseConf.UnixSocketPath == "" {
			if baseConf.BindAddr == "" {
				baseConf.BindAddr = "127.0.0.1"
			}
			if baseConf.Port == 0 {
				baseConf.Port = syslogConf.DefaultPort()
			}
		}

		if baseConf.Format == "" {
			baseConf.Format = "auto"
		}
		if filterConf.TopicTmpl == "" {
			filterConf.TopicTmpl = "rsyslog-{{.Appname}}"
		}
		if filterConf.PartitionTmpl == "" {
			filterConf.PartitionTmpl = "mypk-{{.Hostname}}"
		}
		if baseConf.KeepAlivePeriod == 0 {
			baseConf.KeepAlivePeriod = 75 * time.Second
		}
		if baseConf.Timeout == 0 {
			baseConf.Timeout = time.Minute
		}
		if baseConf.Encoding == "" {
			baseConf.Encoding = "utf8"
		}

		if len(filterConf.TopicTmpl) > 0 {
			_, err = template.New("topic").Parse(filterConf.TopicTmpl)
			if err != nil {
				return ConfigurationCheckError{ErrString: "Error compiling the topic template", Err: err}
			}
		}
		if len(filterConf.PartitionTmpl) > 0 {
			_, err = template.New("partition").Parse(filterConf.PartitionTmpl)
			if err != nil {
				return ConfigurationCheckError{ErrString: "Error compiling the partition key template", Err: err}
			}
		}

		_, err = baseConf.GetListenAddr()
		if err != nil {
			return ConfigurationCheckError{Err: err}
		}

		switch baseConf.Format {
		case "rfc5424", "rfc3164", "json", "auto":
		default:
			if _, ok := parsersNames[baseConf.Format]; !ok {
				return ConfigurationCheckError{ErrString: fmt.Sprintf("Unknown parser: '%s'", baseConf.Format)}
			}
		}
		syslogConf.SetConfID()
	}

	if c.Journald.Enabled {
		var err error

		if len(c.Journald.TopicTmpl) > 0 {
			_, err = template.New("journaldtopic").Parse(c.Journald.TopicTmpl)
			if err != nil {
				return ConfigurationCheckError{ErrString: "Error compiling the topic template", Err: err}
			}
		}
		if len(c.Journald.PartitionTmpl) > 0 {
			_, err = template.New("journaldpartition").Parse(c.Journald.PartitionTmpl)
			if err != nil {
				return ConfigurationCheckError{ErrString: "Error compiling the partition key template", Err: err}
			}
		}
	}
	if r != nil {
		m, err := r.GetBoxSecret()
		if err != nil {
			return ConfigurationCheckError{ErrString: "Failed to retrieve the current session encryption secret", Err: err}
		}
		defer m.Destroy()
		err = c.Store.EncryptSecret(m)
		if err != nil {
			return ConfigurationCheckError{ErrString: "Failed to encrypt the Store secret", Err: err}
		}
	} else {
		c.Store.Secret = ""
	}

	c.Journald.SetConfID()
	c.Accounting.SetConfID()
	c.KafkaDest.Partitioner = strings.TrimSpace(strings.ToLower(c.KafkaDest.Partitioner))
	c.KafkaDest.Partitioner = strings.Replace(c.KafkaDest.Partitioner, "-", "", -1)
	c.KafkaDest.Partitioner = strings.Replace(c.KafkaDest.Partitioner, "_", "", -1)

	return nil
}
