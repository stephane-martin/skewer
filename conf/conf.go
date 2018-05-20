package conf

//go:generate goderive .

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Shopify/sarama"
	"github.com/bsm/sarama-cluster"
	"github.com/fatih/set"
	"github.com/nats-io/go-nats"
	metrics "github.com/rcrowley/go-metrics"

	"github.com/BurntSushi/toml"
	"github.com/inconshreveable/log15"
	"github.com/spf13/viper"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/decoders/base"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

var Version string
var GitCommit string

func (c BaseConfig) Clone() BaseConfig {
	return deriveCloneBaseConfig(c)
}

func NewBaseConf() BaseConfig {
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

		KafkaDest: &KafkaDestConfig{
			KafkaBaseConfig: KafkaBaseConfig{
				Brokers: []string{},
			},
		},

		NATSDest: &NATSDestConfig{
			NServers: []string{},
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
		return baseConf, confSyntaxError(err, "")
	}
	err = baseConf.Complete(nil)
	if err != nil {
		return baseConf, confSyntaxError(err, "")
	}
	return baseConf, nil
}

func (c *BaseConfig) String() (string, error) {
	return c.Export()
}

type KafkaVersion [4]int

//noinspection GoSnakeCaseUsage
var V0_8_2_0 = KafkaVersion{0, 8, 2, 0}

//noinspection GoSnakeCaseUsage
var V0_8_2_1 = KafkaVersion{0, 8, 2, 1}

//noinspection GoSnakeCaseUsage
var V0_8_2_2 = KafkaVersion{0, 8, 2, 2}

//noinspection GoSnakeCaseUsage
var V0_9_0_0 = KafkaVersion{0, 9, 0, 0}

//noinspection GoSnakeCaseUsage
var V0_9_0_1 = KafkaVersion{0, 9, 0, 1}

//noinspection GoSnakeCaseUsage
var V0_10_0_0 = KafkaVersion{0, 10, 0, 0}

//noinspection GoSnakeCaseUsage
var V0_10_0_1 = KafkaVersion{0, 10, 0, 1}

//noinspection GoSnakeCaseUsage
var V0_10_1_0 = KafkaVersion{0, 10, 1, 0}

//noinspection GoSnakeCaseUsage
var V0_10_2_0 = KafkaVersion{0, 10, 2, 0}

//noinspection GoSnakeCaseUsage
var V0_11_0_0 = KafkaVersion{0, 11, 0, 0}

//noinspection GoSnakeCaseUsage
var V1_0_0_0 = KafkaVersion{1, 0, 0, 0}

func ParseVersion(v string) (skv sarama.KafkaVersion, e error) {
	var ver KafkaVersion
	for i, n := range strings.SplitN(v, ".", 4) {
		ver[i], e = strconv.Atoi(n)
		if e != nil {
			return skv, confCheckError(
				eerrors.WithTags(
					eerrors.Wrap(e, "Invalid format for kafka version"),
					"version", v,
				),
			)
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
	return v, confCheckError(eerrors.New("Minimal Kafka version is 0.8.2.0"))
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

func (c *FilterSubConfig) CalculateID() utils.MyULID {
	return utils.MyULID(string(fnv.New128a().Sum([]byte(c.Export()))))
}

func (c *HTTPServerSourceConfig) SetConfID() {
	c.ConfID = c.FilterSubConfig.CalculateID()
}

func (c *TCPSourceConfig) SetConfID() {
	c.ConfID = c.FilterSubConfig.CalculateID()
}

func (c *UDPSourceConfig) SetConfID() {
	c.ConfID = c.FilterSubConfig.CalculateID()
}

func (c *RELPSourceConfig) SetConfID() {
	c.ConfID = c.FilterSubConfig.CalculateID()
}

func (c *DirectRELPSourceConfig) SetConfID() {
	c.ConfID = c.FilterSubConfig.CalculateID()
}

func (c *GraylogSourceConfig) SetConfID() {
	c.ConfID = c.FilterSubConfig.CalculateID()
}

func (c *JournaldConfig) SetConfID() {
	c.ConfID = c.FilterSubConfig.CalculateID()
}

func (c *AccountingSourceConfig) SetConfID() {
	c.ConfID = c.FilterSubConfig.CalculateID()
}

func (c *MacOSSourceConfig) SetConfID() {
	c.ConfID = c.FilterSubConfig.CalculateID()
}

func (c *KafkaSourceConfig) SetConfID() {
	c.ConfID = c.FilterSubConfig.CalculateID()
}

func (c *FilesystemSourceConfig) SetConfID() {
	c.ConfID = c.FilterSubConfig.CalculateID()
}

func (c *HTTPServerSourceConfig) GetClientAuthType() tls.ClientAuthType {
	return convertClientAuthType(c.ClientAuthType)
}

func (c *TCPSourceConfig) GetClientAuthType() tls.ClientAuthType {
	return convertClientAuthType(c.ClientAuthType)
}

func (c *HTTPServerDestConfig) GetClientAuthType() tls.ClientAuthType {
	return convertClientAuthType(c.ClientAuthType)
}

func (c *RELPSourceConfig) GetClientAuthType() tls.ClientAuthType {
	return convertClientAuthType(c.ClientAuthType)
}

func (c *DirectRELPSourceConfig) GetClientAuthType() tls.ClientAuthType {
	return convertClientAuthType(c.ClientAuthType)
}

func convertClientAuthType(authType string) tls.ClientAuthType {
	s := strings.TrimSpace(authType)
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

func (c *BaseConfig) GetCertificateFiles() (res map[string][]string) {
	res = make(map[string][]string)
	s := set.New(set.ThreadSafe)
	s.Add(c.KafkaDest.CAFile, c.KafkaDest.CertFile, c.KafkaDest.KeyFile)
	s.Add(c.RELPDest.CAFile, c.RELPDest.CertFile, c.RELPDest.KeyFile)
	s.Add(c.TCPDest.CAFile, c.TCPDest.CertFile, c.TCPDest.KeyFile)
	s.Add(c.HTTPServerDest.CAFile, c.HTTPServerDest.CertFile, c.HTTPServerDest.KeyFile)
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

	s = set.New(set.ThreadSafe)
	for _, src := range c.HTTPServerSource {
		s.Add(src.CAFile, src.CertFile, src.KeyFile)
	}
	res["httpserversource"] = cleanList(s)

	return res
}

func (c *BaseConfig) GetCertificatePaths() (res map[string][]string) {
	res = map[string][]string{}
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

func (c *ListenersConfig) GetListenAddrs() (addrs map[int]string, err error) {
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

func (c *TCPSourceConfig) Export() string {
	b, _ := json.Marshal(c)
	return string(b)
}

func (c *UDPSourceConfig) Export() string {
	b, _ := json.Marshal(c)
	return string(b)
}

func (c *GraylogSourceConfig) Export() string {
	b, _ := json.Marshal(c)
	return string(b)
}

func (c *RELPSourceConfig) Export() string {
	b, _ := json.Marshal(c)
	return string(b)
}

func (c *DirectRELPSourceConfig) Export() string {
	b, _ := json.Marshal(c)
	return string(b)
}

func (c *FilterSubConfig) Export() string {
	b, _ := json.Marshal(c)
	return string(b)
}

func ImportSyslogConfig(data []byte) (*FilterSubConfig, error) {
	c := FilterSubConfig{}
	err := json.Unmarshal(data, &c)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal the syslog config: %s", err.Error())
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
			return nil, eerrors.Wrap(err, "Error building the TLS configuration for Kafka")
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
			return nil, eerrors.Wrap(err, "Error building the TLS configuration for Kafka")
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

	// TODO: MetricRegistry ?
	return s, nil
}

func (c *KafkaDestConfig) GetAsyncProducer(confined bool) (sarama.AsyncProducer, error) {
	conf, err := c.GetSaramaProducerConfig(confined)
	if err != nil {
		return nil, err
	}
	p, err := sarama.NewAsyncProducer(c.Brokers, conf)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (c *KafkaDestConfig) GetClient(confined bool) (sarama.Client, error) {
	conf, err := c.GetSaramaProducerConfig(confined)
	if err != nil {
		return nil, err
	}
	cl, err := sarama.NewClient(c.Brokers, conf)
	if err != nil {
		return nil, err
	}
	return cl, nil
}

func (c *KafkaSourceConfig) GetClient(confined bool) (*cluster.Consumer, metrics.Registry, error) {
	conf, err := c.GetSaramaConsumerConfig(confined)
	if err != nil {
		return nil, nil, err
	}
	cl, err := cluster.NewConsumer(c.Brokers, c.GroupID, c.Topics, conf)
	if err != nil {
		return nil, nil, err
	}
	return cl, conf.Config.MetricRegistry, nil
}

func getViper(confDir string) (v *viper.Viper, err error) {
	v = viper.New()
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
		return nil, err
	}
	return v, nil
}

func InitLoad(ctx context.Context, confDir string, p consul.ConnParams, r kring.Ring, l log15.Logger) (c BaseConfig, updates chan *BaseConfig, err error) {
	defer func() {
		// viper may panic... let's catch that
		if e := eerrors.Err(recover()); e != nil {
			l.Warn("Panic recovered in conf.InitLoad()", "error", e.Error())
		}
		if err != nil {
			c = NewBaseConf()
			updates = nil
		}
	}()

	var firstResults map[string]string
	c = NewBaseConf()

	v, err := getViper(confDir)
	if err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError:
			l.Info("No configuration file was found")
		default:
			return c, nil, confReadError(err, confDir)
		}
	}

	var watchCtx context.Context
	var cancelWatch context.CancelFunc
	var consulResults chan map[string]string
	consulAddr := strings.TrimSpace(p.Address)

	if len(consulAddr) > 0 {
		l.Info("Reading conf from Consul", "addr", consulAddr)
		consulResults = func(v *viper.Viper) (consulUpdates chan map[string]string) {
			clt, err := consul.NewClient(p)
			if err != nil {
				l.Error("Error creating Consul client: configuration will not be fetched from Consul", "error", err)
				return
			}

			consulUpdates = make(chan map[string]string, 10)
			watchCtx, cancelWatch = context.WithCancel(ctx)
			firstResults, err = consul.Watch(
				consul.Logger(l),
				consul.Client(clt),
				consul.Context(watchCtx),
				consul.Prefix(p.Key),
				consul.ResultsChan(consulUpdates),
				consul.Recursive(false),
			)

			if err != nil {
				l.Error("Error reading from Consul", "error", err)
				cancelWatch()
				consulUpdates = nil
				return
			}

			err = FromConsul(v, getFirstValue(firstResults))
			if err != nil {
				l.Error("Error decoding configuration from Consul", "error", err)
				return
			}
			return
		}(v)
	}

	err = v.Unmarshal(&c)
	if err != nil {
		return NewBaseConf(), nil, confSyntaxError(err, v.ConfigFileUsed())
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
		updates = make(chan *BaseConfig)
		go func() {
		Loop:
			for result := range consulResults {
				v, err := getViper(confDir)
				if err != nil {
					switch err.(type) {
					case viper.ConfigFileNotFoundError:
						l.Info("No configuration file was found")
					default:
						l.Warn("Error reading configuration file", "error", err)
						continue Loop
					}
				}

				err = FromConsul(v, getFirstValue(result))
				if err != nil {
					l.Warn("Error decoding conf from Consul", "error", err)
					continue Loop
				}

				newConfig := NewBaseConf()
				err = v.Unmarshal(&newConfig)
				if err != nil {
					l.Warn("Error unmarshaling new configuration", "error", err)
					continue Loop
				}

				err = newConfig.Complete(r)
				if err != nil {
					l.Error("Error updating conf from Consul", "error", err)
					continue Loop
				}
				updates <- &newConfig
			}
			close(updates)
		}()
	}

	return c, updates, nil
}

func getFirstValue(m map[string]string) (val string) {
	for _, val = range m {
		break
	}
	return val
}

func FromConsul(v *viper.Viper, confStr string) (err error) {
	defer func() {
		// sometimes viper panics... let's catch that
		if e := eerrors.Err(recover()); e != nil {
			err = e
		}
	}()
	return v.MergeConfig(strings.NewReader(confStr))
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
		if base.ParseFormat(parserConf.Name) != -1 {
			return confCheckError(eerrors.New("Parser configuration must not use a reserved name"))
		}
		if name == "" {
			return confCheckError(eerrors.New("Empty parser name"))
		}
		if _, ok := parsersNames[name]; ok {
			return confCheckError(eerrors.New("The same parser name is used multiple times"))
		}
		f := strings.TrimSpace(parserConf.Func)
		if len(f) == 0 {
			return confCheckError(eerrors.New("Empty parser func"))
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
		return confCheckError(
			eerrors.Wrap(err, "Kafka version can not be parsed"),
		)
	}

	if len(c.NATSDest.NServers) == 0 {
		if c.NATSDest.TLSEnabled {
			return confCheckError(
				eerrors.New("NATS servers must be explicitly configured when TLS is enabled"),
			)
		}
		c.NATSDest.NServers = []string{nats.DefaultURL}
	}

	for i := range c.NATSDest.NServers {
		c.NATSDest.NServers[i] = strings.TrimSpace(c.NATSDest.NServers[i])
		if !strings.HasPrefix(c.NATSDest.NServers[i], "tls://") && !strings.HasPrefix(c.NATSDest.NServers[i], "nats://") {
			return confCheckError(eerrors.New("Every NATS server must start with tls:// or nats://"))
		}
		if c.NATSDest.TLSEnabled && !strings.HasPrefix(c.NATSDest.NServers[i], "tls") {
			return confCheckError(eerrors.New("TLS is enabled for NATS, but a server lacks the tls:// prefix"))
		}
		if !c.NATSDest.TLSEnabled && !strings.HasPrefix(c.NATSDest.NServers[i], "nats") {
			return confCheckError(eerrors.New("TLS is not enabled for NATS, but a server lacks the nats:// prefix"))
		}
	}

	sources := make([]Source, 0)
	for i := range c.FSSource {
		sources = append(sources, &c.FSSource[i])
	}
	for i := range c.TCPSource {
		sources = append(sources, &c.TCPSource[i])
	}
	for i := range c.UDPSource {
		sources = append(sources, &c.UDPSource[i])
	}
	for i := range c.RELPSource {
		sources = append(sources, &c.RELPSource[i])
	}
	for i := range c.DirectRELPSource {
		sources = append(sources, &c.DirectRELPSource[i])
	}
	for i := range c.GraylogSource {
		sources = append(sources, &c.GraylogSource[i])
	}
	for i := range c.KafkaSource {
		sources = append(sources, &c.KafkaSource[i])
	}
	for i := range c.HTTPServerSource {
		sources = append(sources, &c.HTTPServerSource[i])
	}
	sources = append(sources, &c.Journald, &c.Accounting, &c.MacOS)

	for i := range c.TCPSource {
		if len(c.TCPSource[i].FrameDelimiter) == 0 {
			c.TCPSource[i].FrameDelimiter = "\n"
		}
	}

	// set default values for http server sources
	for i := range c.HTTPServerSource {
		hc := &c.HTTPServerSource[i]
		if hc.BindAddr == "" {
			hc.BindAddr = "127.0.0.1"
		}
		if hc.Port == 0 {
			hc.Port = hc.DefaultPort()
		}
		if hc.ConnKeepAlivePeriod == 0 {
			hc.ConnKeepAlivePeriod = 3 * time.Minute
		}
		if hc.MaxHeaderBytes == 0 {
			hc.MaxHeaderBytes = http.DefaultMaxHeaderBytes
		}
		if hc.IdleTimeout == 0 {
			hc.IdleTimeout = 2 * time.Minute
		}
		if len(hc.FrameDelimiter) == 0 {
			hc.FrameDelimiter = "\n"
		}
		if hc.MaxBodySize == 0 {
			hc.MaxBodySize = 10 * 1024 * 1024
		}
		if len(hc.DecoderBaseConfig.Charset) == 0 {
			hc.DecoderBaseConfig.Charset = "utf8"
		}
		if len(hc.DecoderBaseConfig.Format) == 0 {
			hc.DecoderBaseConfig.Format = "json"
		}
		if hc.MaxMessages == 0 {
			hc.MaxMessages = 10000
		}
	}

	// set default values for sources
	for _, sourceConf := range sources {
		listeners := sourceConf.ListenersConf()
		filtering := sourceConf.FilterConf()
		decodr := sourceConf.DecoderConf()
		if decodr != nil {
			if decodr.Format == "" {
				decodr.Format = "rfc5424"
			}
			if decodr.Charset == "" {
				decodr.Charset = "utf8"
			}
		}
		if listeners != nil {
			if listeners.UnixSocketPath == "" {
				if listeners.BindAddr == "" {
					listeners.BindAddr = "127.0.0.1"
				}
				if len(listeners.Ports) == 0 {
					listeners.Ports = []int{sourceConf.DefaultPort()}
				}
			}

			if listeners.Timeout <= 0 {
				listeners.Timeout = time.Minute
			}

			if listeners.KeepAlivePeriod <= 0 {
				listeners.KeepAlivePeriod = 75 * time.Second
			}
			_, err = listeners.GetListenAddrs()
			if err != nil {
				return confCheckError(err)
			}

		}
		if filtering != nil {
			if filtering.TopicTmpl == "" {
				filtering.TopicTmpl = "topic-{{.AppName}}"
			}
			if filtering.PartitionTmpl == "" {
				filtering.PartitionTmpl = "partition-{{.HostName}}"
			}

			if len(filtering.TopicTmpl) > 0 {
				_, err = template.New("topic").Parse(filtering.TopicTmpl)
				if err != nil {
					return confCheckError(
						eerrors.Wrap(err, "Error compiling topic template"),
					)
				}
			}
			if len(filtering.PartitionTmpl) > 0 {
				_, err = template.New("partition").Parse(filtering.PartitionTmpl)
				if err != nil {
					return confCheckError(
						eerrors.Wrap(err, "Error compiling the partition key template"),
					)
				}
			}
			sourceConf.SetConfID()
		}

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
		if len(conf.ClientID) == 0 {
			conf.ClientID = "skewers"
		}
		if len(conf.Version) == 0 {
			conf.Version = "0.10.1.0"
		}
		_, err = ParseVersion(conf.Version)
		if err != nil {
			return confCheckError(eerrors.Wrap(err, "Kafka version can not be parsed"))
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
		conf.SetConfID()
	}

	if r != nil {
		m, err := r.GetBoxSecret()
		if err != nil {
			return eerrors.Wrap(err, "Failed to retrieve the current session encryption secret")
		}
		defer m.Destroy()
		err = c.Store.EncryptSecret(m)
		if err != nil {
			return eerrors.Wrap(err, "Failed to encrypt the Store secret")
		}
	} else {
		c.Store.Secret = ""
	}

	c.KafkaDest.Partitioner = strings.TrimSpace(strings.ToLower(c.KafkaDest.Partitioner))
	c.KafkaDest.Partitioner = strings.Replace(c.KafkaDest.Partitioner, "-", "", -1)
	c.KafkaDest.Partitioner = strings.Replace(c.KafkaDest.Partitioner, "_", "", -1)

	return nil
}
