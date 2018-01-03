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
	"strconv"
	"strings"
	"text/template"
	"time"

	sarama "github.com/Shopify/sarama"
	cluster "github.com/bsm/sarama-cluster"
	"github.com/fatih/set"

	"github.com/BurntSushi/toml"
	"github.com/hashicorp/errwrap"
	"github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/model/decoders"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/utils"
)

var Version string
var GitCommit string

func (source BaseConfig) Clone() BaseConfig {
	//return source
	return deriveCloneBaseConfig(source)
}

func NewBaseConf() BaseConfig {
	brokers := []string{}
	baseConf := BaseConfig{
		TCPSource:        []TCPSourceConfig{},
		UDPSource:        []UDPSourceConfig{},
		RELPSource:       []RELPSourceConfig{},
		DirectRELPSource: []DirectRELPSourceConfig{},
		GraylogSource:    []GraylogSourceConfig{},
		KafkaSource:      []KafkaSourceConfig{},
		Store:            StoreConfig{},
		Parsers:          []ParserConfig{},
		Journald:         JournaldConfig{},
		Metrics:          MetricsConfig{},

		KafkaDest: KafkaDestConfig{
			KafkaBaseConfig: KafkaBaseConfig{
				Brokers: brokers,
			},
		},
	}
	return baseConf
}

func Default() (BaseConfig, error) {
	v := viper.New()
	SetDefaults(v)
	baseConf := NewBaseConf()
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

func (c *BaseConfig) String() (string, error) {
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
var V0_11_0_0 = KafkaVersion{0, 11, 0, 0}
var V1_0_0_0 = KafkaVersion{1, 0, 0, 0}

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
	if l.Greater(V1_0_0_0) {
		return sarama.V1_0_0_0, nil
	}
	if l.Greater(V0_11_0_0) {
		return sarama.V0_11_0_0, nil
	}
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

func (c *TCPSourceConfig) SetConfID() {
	copy(c.ConfID[:], c.FilterSubConfig.CalculateID())
}

func (c *UDPSourceConfig) SetConfID() {
	copy(c.ConfID[:], c.FilterSubConfig.CalculateID())
}

func (c *RELPSourceConfig) SetConfID() {
	copy(c.ConfID[:], c.FilterSubConfig.CalculateID())
}

func (c *DirectRELPSourceConfig) SetConfID() {
	copy(c.ConfID[:], c.FilterSubConfig.CalculateID())
}

func (c *GraylogSourceConfig) SetConfID() {
	copy(c.ConfID[:], c.FilterSubConfig.CalculateID())
}

func (c *JournaldConfig) SetConfID() {
	copy(c.ConfID[:], c.FilterSubConfig.CalculateID())
}

func (c *AccountingConfig) SetConfID() {
	copy(c.ConfID[:], c.FilterSubConfig.CalculateID())
}

func (c *KafkaSourceConfig) SetConfID() {
	copy(c.ConfID[:], c.FilterSubConfig.CalculateID())
}

func (c *TCPSourceConfig) GetClientAuthType() tls.ClientAuthType {
	return convertClientAuthType(c.ClientAuthType)
}

func (c *RELPSourceConfig) GetClientAuthType() tls.ClientAuthType {
	return convertClientAuthType(c.ClientAuthType)
}

func (c *DirectRELPSourceConfig) GetClientAuthType() tls.ClientAuthType {
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

func cleanList(list set.Interface) (res []string) {
	res = make([]string, 0)
	for _, f := range list.List() {
		if f != nil {
			if fs, ok := f.(string); ok && len(fs) > 0 {
				if utils.FileExists(fs) {
					res = append(res, fs)
				}
			}
		}
	}
	return res
}

func (c *BaseConfig) GetCertificateFiles() (res map[string]([]string)) {
	res = map[string]([]string){}
	s := set.New(set.ThreadSafe)
	s.Add(c.KafkaDest.CAFile, c.KafkaDest.CertFile, c.KafkaDest.KeyFile)
	s.Add(c.RELPDest.CAFile, c.RELPDest.CertFile, c.RELPDest.KeyFile)
	s.Add(c.TCPDest.CAFile, c.TCPDest.CertFile, c.TCPDest.KeyFile)
	res["dests"] = cleanList(s)

	s = set.New(set.ThreadSafe)
	for _, src := range c.TCPSource {
		s.Add(src.CAFile, src.CertFile, src.KeyFile)
	}
	res["tcpsource"] = cleanList(s)

	s = set.New(set.ThreadSafe)
	for _, src := range c.RELPSource {
		s.Add(src.CAFile, src.CertFile, src.KeyFile)
	}
	res["relpsource"] = cleanList(s)

	s = set.New(set.ThreadSafe)
	for _, src := range c.DirectRELPSource {
		s.Add(src.CAFile, src.CertFile, src.KeyFile)
	}
	res["directrelpsource"] = cleanList(s)

	s = set.New(set.ThreadSafe)
	for _, src := range c.KafkaSource {
		s.Add(src.CAFile, src.CertFile, src.KeyFile)
	}
	res["kafkasource"] = cleanList(s)

	return res

}

func (c *BaseConfig) GetCertificatePaths() (res map[string]([]string)) {
	res = map[string]([]string){}
	s := set.New(set.ThreadSafe)
	s.Add(c.KafkaDest.CAPath)
	s.Add(c.RELPDest.CAPath)
	s.Add(c.TCPDest.CAPath)
	res["dests"] = cleanList(s)

	s = set.New(set.ThreadSafe)
	for _, src := range c.TCPSource {
		s.Add(src.CAPath)
	}
	res["tcpsource"] = cleanList(s)

	s = set.New(set.ThreadSafe)
	for _, src := range c.RELPSource {
		s.Add(src.CAPath)
	}
	res["relpsource"] = cleanList(s)

	s = set.New(set.ThreadSafe)
	for _, src := range c.DirectRELPSource {
		s.Add(src.CAPath)
	}
	res["directrelpsource"] = cleanList(s)

	s = set.New(set.ThreadSafe)
	for _, src := range c.KafkaSource {
		s.Add(src.CAPath)
	}
	res["kafkasource"] = cleanList(s)

	return res
}

func (c *SyslogSourceBaseConfig) GetListenAddrs() (addrs map[int]string, err error) {
	addrs = map[int]string{}
	if len(c.UnixSocketPath) > 0 {
		return
	}
	bindIP := net.ParseIP(c.BindAddr)
	if bindIP == nil {
		return nil, fmt.Errorf("bind_addr is not an IP address: %s", c.BindAddr)
	}

	if bindIP.IsUnspecified() {
		for _, port := range c.Ports {
			addrs[port] = fmt.Sprintf(":%d", port)
		}
	} else {
		ip := bindIP.String()
		for _, port := range c.Ports {
			addrs[port] = fmt.Sprintf("%s:%d", ip, port)
		}
	}
	return
}

func (c *TCPSourceConfig) Export() []byte {
	b, _ := json.Marshal(c)
	return b
}

func (c *UDPSourceConfig) Export() []byte {
	b, _ := json.Marshal(c)
	return b
}

func (c *GraylogSourceConfig) Export() []byte {
	b, _ := json.Marshal(c)
	return b
}

func (c *RELPSourceConfig) Export() []byte {
	b, _ := json.Marshal(c)
	return b
}

func (c *DirectRELPSourceConfig) Export() []byte {
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

func (c *KafkaSourceConfig) GetSaramaConsumerConfig(confined bool) (*cluster.Config, error) {
	s := cluster.NewConfig()
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

	s.Consumer.Retry.Backoff = c.RetryBackoff
	s.Consumer.Fetch.Default = c.DefaultFetchBytes
	s.Consumer.Fetch.Max = c.MaxFetchBytes
	s.Consumer.Fetch.Min = c.MinFetchBytes
	s.Consumer.MaxWaitTime = c.MaxWaitTime
	s.Consumer.MaxProcessingTime = c.MaxProcessingTime
	s.Consumer.Offsets.CommitInterval = c.OffsetsCommitInterval
	s.Consumer.Offsets.Initial = c.OffsetsInitial
	s.Consumer.Offsets.Retention = c.OffsetsRetention
	s.Consumer.Return.Errors = true

	v, _ := ParseVersion(c.Version) // the ignored error has been checked at launch
	s.Version = v

	if c.TLSEnabled {
		tlsConf, err := utils.NewTLSConfig("", c.CAFile, c.CAPath, c.CertFile, c.KeyFile, c.Insecure, confined)
		if err == nil {
			s.Net.TLS.Enable = true
			s.Net.TLS.Config = tlsConf
		} else {
			return nil, errwrap.Wrapf("Error building the TLS configuration for Kafka: {{err}}", err)
		}

	}

	s.Group.Offsets.Retry.Max = c.OffsetsMaxRetry
	s.Group.Session.Timeout = c.SessionTimeout
	s.Group.Heartbeat.Interval = c.HeartbeatInterval
	s.Group.Return.Notifications = false

	return s, nil
}

func (c *KafkaDestConfig) GetSaramaProducerConfig(confined bool) (*sarama.Config, error) {
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
		tlsConf, err := utils.NewTLSConfig("", c.CAFile, c.CAPath, c.CertFile, c.KeyFile, c.Insecure, confined)
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

func (c *KafkaDestConfig) GetAsyncProducer(confined bool) (sarama.AsyncProducer, error) {
	conf, err := c.GetSaramaProducerConfig(confined)
	if err != nil {
		return nil, err
	}
	p, err := sarama.NewAsyncProducer(c.Brokers, conf)
	if err == nil {
		return p, nil
	}
	return nil, KafkaError{Err: err}
}

func (c *KafkaDestConfig) GetClient(confined bool) (sarama.Client, error) {
	conf, err := c.GetSaramaProducerConfig(confined)
	if err != nil {
		return nil, err
	}
	cl, err := sarama.NewClient(c.Brokers, conf)
	if err == nil {
		return cl, nil
	}
	return nil, KafkaError{Err: err}
}

func (c *KafkaSourceConfig) GetClient(confined bool) (*cluster.Consumer, error) {
	conf, err := c.GetSaramaConsumerConfig(confined)
	if err != nil {
		return nil, err
	}
	cl, err := cluster.NewConsumer(c.Brokers, c.GroupID, c.Topics, conf)
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
			c = NewBaseConf()
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
			return NewBaseConf(), nil, ConfigurationReadError{err}
		case viper.ConfigFileNotFoundError:
			logger.Info("No configuration file was found")
		}
	}

	var baseConf BaseConfig
	err = v.Unmarshal(&baseConf)
	if err != nil {
		return NewBaseConf(), nil, ConfigurationSyntaxError{Err: err, Filename: v.ConfigFileUsed()}
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
		return NewBaseConf(), nil, err
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

	tcpSourceConfs := []TCPSourceConfig{}
	for _, syslogConf := range rawTcpSourceConf {
		vi = viper.New()
		for k, v := range syslogConf {
			vi.Set(k, v)
		}
		sconf := TCPSourceConfig{}
		err := vi.Unmarshal(&sconf)
		if err == nil {
			tcpSourceConfs = append(tcpSourceConfs, sconf)
		} else {
			return err
		}
	}

	udpSourceConfs := []UDPSourceConfig{}
	for _, syslogConf := range rawUdpSourceConf {
		vi = viper.New()
		for k, v := range syslogConf {
			vi.Set(k, v)
		}
		sconf := UDPSourceConfig{}
		err := vi.Unmarshal(&sconf)
		if err == nil {
			udpSourceConfs = append(udpSourceConfs, sconf)
		} else {
			return err
		}
	}

	relpSourceConfs := []RELPSourceConfig{}
	for _, syslogConf := range rawRelpSourceConf {
		vi = viper.New()
		for k, v := range syslogConf {
			vi.Set(k, v)
		}
		sconf := RELPSourceConfig{}
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
	c.TCPSource = append(c.TCPSource, tcpSourceConfs...)
	c.UDPSource = append(c.UDPSource, udpSourceConfs...)
	c.RELPSource = append(c.RELPSource, relpSourceConfs...)
	c.Parsers = append(c.Parsers, parsersConf...)

	return nil
}

func (c *BaseConfig) Export() (string, error) {
	buf := new(bytes.Buffer)
	encoder := toml.NewEncoder(buf)
	err := encoder.Encode(c)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (c *BaseConfig) Complete(r kring.Ring) (err error) {
	parsersNames := map[string]bool{}
	for _, parserConf := range c.Parsers {
		name := strings.TrimSpace(parserConf.Name)
		if decoders.ParseFormat(parserConf.Name) != -1 {
			return ConfigurationCheckError{ErrString: "Parser configuration must not use a reserved name"}
		}
		if name == "" {
			return ConfigurationCheckError{ErrString: "Empty parser name"}
		}
		if _, ok := parsersNames[name]; ok {
			return ConfigurationCheckError{ErrString: "The same parser name is used multiple times"}
		}
		f := strings.TrimSpace(parserConf.Func)
		if len(f) == 0 {
			return ConfigurationCheckError{ErrString: "Empty parser func"}
		}
		parsersNames[name] = true
	}

	_, err = c.Main.GetDestinations()
	if err != nil {
		return err
	}

	err = c.CheckDestinations()
	if err != nil {
		return err
	}

	_, err = ParseVersion(c.KafkaDest.Version)
	if err != nil {
		return ConfigurationCheckError{ErrString: "Kafka version can't be parsed", Err: err}
	}

	syslogConfs := []SyslogSourceConfig{}
	for i := range c.TCPSource {
		syslogConfs = append(syslogConfs, &c.TCPSource[i])
	}
	for i := range c.UDPSource {
		syslogConfs = append(syslogConfs, &c.UDPSource[i])
	}
	for i := range c.RELPSource {
		syslogConfs = append(syslogConfs, &c.RELPSource[i])
	}
	for i := range c.DirectRELPSource {
		syslogConfs = append(syslogConfs, &c.DirectRELPSource[i])
	}
	for i := range c.GraylogSource {
		syslogConfs = append(syslogConfs, &c.GraylogSource[i])
	}

	// set default values for TCP, UDP, Graylog and RELP sources.
	for i := range c.TCPSource {
		if len(c.TCPSource[i].FrameDelimiter) == 0 {
			c.TCPSource[i].FrameDelimiter = "\n"
		}
	}

	for _, syslogConf := range syslogConfs {
		baseConf := syslogConf.GetSyslogConf()
		filterConf := syslogConf.GetFilterConf()

		if baseConf.UnixSocketPath == "" {
			if baseConf.BindAddr == "" {
				baseConf.BindAddr = "127.0.0.1"
			}
			if len(baseConf.Ports) == 0 {
				baseConf.Ports = []int{syslogConf.DefaultPort()}
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
		if baseConf.KeepAlivePeriod <= 0 {
			baseConf.KeepAlivePeriod = 75 * time.Second
		}
		if baseConf.Timeout <= 0 {
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
		_, err = baseConf.GetListenAddrs()
		if err != nil {
			return ConfigurationCheckError{Err: err}
		}

		if decoders.ParseFormat(baseConf.Format) == -1 {
			if _, ok := parsersNames[baseConf.Format]; !ok {
				return ConfigurationCheckError{ErrString: fmt.Sprintf("Unknown parser: '%s'", baseConf.Format)}
			}
		}

		syslogConf.SetConfID()
	}

	// set default paramaters for kafka sources
	for i := range c.KafkaSource {
		conf := &(c.KafkaSource[i])
		if len(conf.GroupID) == 0 {
			conf.GroupID = "skewer"
		}
		if conf.OffsetsMaxRetry <= 0 {
			conf.OffsetsMaxRetry = 3
		}
		if conf.HeartbeatInterval <= 0 {
			conf.HeartbeatInterval = 3 * time.Second
		}
		if conf.SessionTimeout <= 0 {
			conf.SessionTimeout = 30 * time.Second
		}
		if len(conf.Format) == 0 {
			conf.Format = "fulljson"
		}
		if len(conf.Encoding) == 0 {
			conf.Encoding = "utf8"
		}
		if len(conf.ClientID) == 0 {
			conf.ClientID = "skewers"
		}
		if len(conf.Version) == 0 {
			conf.Version = "0.10.1.0"
		}
		_, err = ParseVersion(conf.Version)
		if err != nil {
			return ConfigurationCheckError{ErrString: "Kafka version can't be parsed", Err: err}
		}
		if conf.ChannelBufferSize == 0 {
			conf.ChannelBufferSize = 256
		}
		if conf.MaxOpenRequests == 0 {
			conf.MaxOpenRequests = 5
		}
		if conf.DialTimeout == 0 {
			conf.DialTimeout = 30 * time.Second
		}
		if conf.ReadTimeout == 0 {
			conf.ReadTimeout = 30 * time.Second
		}
		if conf.WriteTimeout == 0 {
			conf.WriteTimeout = 30 * time.Second
		}
		if conf.MetadataRetryMax == 0 {
			conf.MetadataRetryMax = 3
		}
		if conf.MetadataRetryBackoff == 0 {
			conf.MetadataRetryBackoff = 250 * time.Millisecond
		}
		if conf.MetadataRefreshFrequency == 0 {
			conf.MetadataRefreshFrequency = 10 * time.Minute
		}
		if conf.RetryBackoff == 0 {
			conf.RetryBackoff = 2 * time.Second
		}
		if conf.MinFetchBytes == 0 {
			conf.MinFetchBytes = 1
		}
		if conf.DefaultFetchBytes == 0 {
			conf.DefaultFetchBytes = 32768
		}
		if conf.MaxFetchBytes == 0 {
			conf.MaxFetchBytes = sarama.MaxResponseSize
		}
		if conf.MaxWaitTime == 0 {
			conf.MaxWaitTime = 250 * time.Millisecond
		}
		if conf.MaxProcessingTime == 0 {
			conf.MaxProcessingTime = 100 * time.Millisecond
		}
		if conf.OffsetsCommitInterval == 0 {
			conf.OffsetsCommitInterval = time.Second
		}
		if conf.OffsetsInitial == 0 {
			conf.OffsetsInitial = sarama.OffsetOldest
		}

		if decoders.ParseFormat(conf.Format) == -1 {
			if _, ok := parsersNames[conf.Format]; !ok {
				return ConfigurationCheckError{ErrString: fmt.Sprintf("Unknown parser: '%s'", conf.Format)}
			}
		}

		if conf.TopicTmpl == "" {
			conf.TopicTmpl = "rsyslog-{{.Appname}}"
		}
		if conf.PartitionTmpl == "" {
			conf.PartitionTmpl = "mypk-{{.Hostname}}"
		}
		if len(conf.TopicTmpl) > 0 {
			_, err = template.New("topic").Parse(conf.TopicTmpl)
			if err != nil {
				return ConfigurationCheckError{ErrString: "Error compiling the topic template", Err: err}
			}
		}
		if len(conf.PartitionTmpl) > 0 {
			_, err = template.New("partition").Parse(conf.PartitionTmpl)
			if err != nil {
				return ConfigurationCheckError{ErrString: "Error compiling the partition key template", Err: err}
			}
		}

		conf.SetConfID()
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
