// Code generated by goderive DO NOT EDIT.

package conf

// deriveCloneBaseConfig returns a clone of the src parameter.
func deriveCloneBaseConfig(src BaseConfig) BaseConfig {
	dst := new(BaseConfig)
	deriveDeepCopy(dst, &src)
	return *dst
}

// deriveDeepCopy recursively copies the contents of src into dst.
func deriveDeepCopy(dst, src *BaseConfig) {
	if src.Syslog == nil {
		dst.Syslog = nil
	} else {
		if dst.Syslog != nil {
			if len(src.Syslog) > len(dst.Syslog) {
				if cap(dst.Syslog) >= len(src.Syslog) {
					dst.Syslog = (dst.Syslog)[:len(src.Syslog)]
				} else {
					dst.Syslog = make([]SyslogConfig, len(src.Syslog))
				}
			} else if len(src.Syslog) < len(dst.Syslog) {
				dst.Syslog = (dst.Syslog)[:len(src.Syslog)]
			}
		} else {
			dst.Syslog = make([]SyslogConfig, len(src.Syslog))
		}
		copy(dst.Syslog, src.Syslog)
	}
	field := new(KafkaConfig)
	deriveDeepCopy_(field, &src.Kafka)
	dst.Kafka = *field
	dst.Store = src.Store
	if src.Parsers == nil {
		dst.Parsers = nil
	} else {
		if dst.Parsers != nil {
			if len(src.Parsers) > len(dst.Parsers) {
				if cap(dst.Parsers) >= len(src.Parsers) {
					dst.Parsers = (dst.Parsers)[:len(src.Parsers)]
				} else {
					dst.Parsers = make([]ParserConfig, len(src.Parsers))
				}
			} else if len(src.Parsers) < len(dst.Parsers) {
				dst.Parsers = (dst.Parsers)[:len(src.Parsers)]
			}
		} else {
			dst.Parsers = make([]ParserConfig, len(src.Parsers))
		}
		copy(dst.Parsers, src.Parsers)
	}
	dst.Journald = src.Journald
	dst.Metrics = src.Metrics
	dst.DirectRelp = src.DirectRelp
}

// deriveDeepCopy_ recursively copies the contents of src into dst.
func deriveDeepCopy_(dst, src *KafkaConfig) {
	if src.Brokers == nil {
		dst.Brokers = nil
	} else {
		if dst.Brokers != nil {
			if len(src.Brokers) > len(dst.Brokers) {
				if cap(dst.Brokers) >= len(src.Brokers) {
					dst.Brokers = (dst.Brokers)[:len(src.Brokers)]
				} else {
					dst.Brokers = make([]string, len(src.Brokers))
				}
			} else if len(src.Brokers) < len(dst.Brokers) {
				dst.Brokers = (dst.Brokers)[:len(src.Brokers)]
			}
		} else {
			dst.Brokers = make([]string, len(src.Brokers))
		}
		copy(dst.Brokers, src.Brokers)
	}
	dst.ClientID = src.ClientID
	dst.Version = src.Version
	dst.ChannelBufferSize = src.ChannelBufferSize
	dst.MaxOpenRequests = src.MaxOpenRequests
	dst.DialTimeout = src.DialTimeout
	dst.ReadTimeout = src.ReadTimeout
	dst.WriteTimeout = src.WriteTimeout
	dst.KeepAlive = src.KeepAlive
	dst.MetadataRetryMax = src.MetadataRetryMax
	dst.MetadataRetryBackoff = src.MetadataRetryBackoff
	dst.MetadataRefreshFrequency = src.MetadataRefreshFrequency
	dst.MessageBytesMax = src.MessageBytesMax
	dst.RequiredAcks = src.RequiredAcks
	dst.ProducerTimeout = src.ProducerTimeout
	dst.Compression = src.Compression
	dst.FlushBytes = src.FlushBytes
	dst.FlushMessages = src.FlushMessages
	dst.FlushFrequency = src.FlushFrequency
	dst.FlushMessagesMax = src.FlushMessagesMax
	dst.RetrySendMax = src.RetrySendMax
	dst.RetrySendBackoff = src.RetrySendBackoff
	dst.TLSEnabled = src.TLSEnabled
	dst.CAFile = src.CAFile
	dst.CAPath = src.CAPath
	dst.KeyFile = src.KeyFile
	dst.CertFile = src.CertFile
	dst.Insecure = src.Insecure
	dst.Partitioner = src.Partitioner
}
