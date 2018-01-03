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
	if src.TCPSource == nil {
		dst.TCPSource = nil
	} else {
		if dst.TCPSource != nil {
			if len(src.TCPSource) > len(dst.TCPSource) {
				if cap(dst.TCPSource) >= len(src.TCPSource) {
					dst.TCPSource = (dst.TCPSource)[:len(src.TCPSource)]
				} else {
					dst.TCPSource = make([]TCPSourceConfig, len(src.TCPSource))
				}
			} else if len(src.TCPSource) < len(dst.TCPSource) {
				dst.TCPSource = (dst.TCPSource)[:len(src.TCPSource)]
			}
		} else {
			dst.TCPSource = make([]TCPSourceConfig, len(src.TCPSource))
		}
		deriveDeepCopy_(dst.TCPSource, src.TCPSource)
	}
	if src.UDPSource == nil {
		dst.UDPSource = nil
	} else {
		if dst.UDPSource != nil {
			if len(src.UDPSource) > len(dst.UDPSource) {
				if cap(dst.UDPSource) >= len(src.UDPSource) {
					dst.UDPSource = (dst.UDPSource)[:len(src.UDPSource)]
				} else {
					dst.UDPSource = make([]UDPSourceConfig, len(src.UDPSource))
				}
			} else if len(src.UDPSource) < len(dst.UDPSource) {
				dst.UDPSource = (dst.UDPSource)[:len(src.UDPSource)]
			}
		} else {
			dst.UDPSource = make([]UDPSourceConfig, len(src.UDPSource))
		}
		deriveDeepCopy_1(dst.UDPSource, src.UDPSource)
	}
	if src.RELPSource == nil {
		dst.RELPSource = nil
	} else {
		if dst.RELPSource != nil {
			if len(src.RELPSource) > len(dst.RELPSource) {
				if cap(dst.RELPSource) >= len(src.RELPSource) {
					dst.RELPSource = (dst.RELPSource)[:len(src.RELPSource)]
				} else {
					dst.RELPSource = make([]RELPSourceConfig, len(src.RELPSource))
				}
			} else if len(src.RELPSource) < len(dst.RELPSource) {
				dst.RELPSource = (dst.RELPSource)[:len(src.RELPSource)]
			}
		} else {
			dst.RELPSource = make([]RELPSourceConfig, len(src.RELPSource))
		}
		deriveDeepCopy_2(dst.RELPSource, src.RELPSource)
	}
	if src.DirectRELPSource == nil {
		dst.DirectRELPSource = nil
	} else {
		if dst.DirectRELPSource != nil {
			if len(src.DirectRELPSource) > len(dst.DirectRELPSource) {
				if cap(dst.DirectRELPSource) >= len(src.DirectRELPSource) {
					dst.DirectRELPSource = (dst.DirectRELPSource)[:len(src.DirectRELPSource)]
				} else {
					dst.DirectRELPSource = make([]DirectRELPSourceConfig, len(src.DirectRELPSource))
				}
			} else if len(src.DirectRELPSource) < len(dst.DirectRELPSource) {
				dst.DirectRELPSource = (dst.DirectRELPSource)[:len(src.DirectRELPSource)]
			}
		} else {
			dst.DirectRELPSource = make([]DirectRELPSourceConfig, len(src.DirectRELPSource))
		}
		deriveDeepCopy_3(dst.DirectRELPSource, src.DirectRELPSource)
	}
	if src.KafkaSource == nil {
		dst.KafkaSource = nil
	} else {
		if dst.KafkaSource != nil {
			if len(src.KafkaSource) > len(dst.KafkaSource) {
				if cap(dst.KafkaSource) >= len(src.KafkaSource) {
					dst.KafkaSource = (dst.KafkaSource)[:len(src.KafkaSource)]
				} else {
					dst.KafkaSource = make([]KafkaSourceConfig, len(src.KafkaSource))
				}
			} else if len(src.KafkaSource) < len(dst.KafkaSource) {
				dst.KafkaSource = (dst.KafkaSource)[:len(src.KafkaSource)]
			}
		} else {
			dst.KafkaSource = make([]KafkaSourceConfig, len(src.KafkaSource))
		}
		deriveDeepCopy_4(dst.KafkaSource, src.KafkaSource)
	}
	if src.GraylogSource == nil {
		dst.GraylogSource = nil
	} else {
		if dst.GraylogSource != nil {
			if len(src.GraylogSource) > len(dst.GraylogSource) {
				if cap(dst.GraylogSource) >= len(src.GraylogSource) {
					dst.GraylogSource = (dst.GraylogSource)[:len(src.GraylogSource)]
				} else {
					dst.GraylogSource = make([]GraylogSourceConfig, len(src.GraylogSource))
				}
			} else if len(src.GraylogSource) < len(dst.GraylogSource) {
				dst.GraylogSource = (dst.GraylogSource)[:len(src.GraylogSource)]
			}
		} else {
			dst.GraylogSource = make([]GraylogSourceConfig, len(src.GraylogSource))
		}
		deriveDeepCopy_5(dst.GraylogSource, src.GraylogSource)
	}
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
	dst.Accounting = src.Accounting
	dst.Main = src.Main
	if src.KafkaDest == nil {
		dst.KafkaDest = nil
	} else {
		dst.KafkaDest = new(KafkaDestConfig)
		deriveDeepCopy_6(dst.KafkaDest, src.KafkaDest)
	}
	dst.UDPDest = src.UDPDest
	dst.TCPDest = src.TCPDest
	dst.HTTPDest = src.HTTPDest
	if src.NATSDest == nil {
		dst.NATSDest = nil
	} else {
		dst.NATSDest = new(NATSDestConfig)
		deriveDeepCopy_7(dst.NATSDest, src.NATSDest)
	}
	dst.RELPDest = src.RELPDest
	dst.FileDest = src.FileDest
	dst.StderrDest = src.StderrDest
	dst.GraylogDest = src.GraylogDest
}

// deriveDeepCopy_ recursively copies the contents of src into dst.
func deriveDeepCopy_(dst, src []TCPSourceConfig) {
	for src_i, src_value := range src {
		field := new(TCPSourceConfig)
		deriveDeepCopy_8(field, &src_value)
		dst[src_i] = *field
	}
}

// deriveDeepCopy_1 recursively copies the contents of src into dst.
func deriveDeepCopy_1(dst, src []UDPSourceConfig) {
	for src_i, src_value := range src {
		field := new(UDPSourceConfig)
		deriveDeepCopy_9(field, &src_value)
		dst[src_i] = *field
	}
}

// deriveDeepCopy_2 recursively copies the contents of src into dst.
func deriveDeepCopy_2(dst, src []RELPSourceConfig) {
	for src_i, src_value := range src {
		field := new(RELPSourceConfig)
		deriveDeepCopy_10(field, &src_value)
		dst[src_i] = *field
	}
}

// deriveDeepCopy_3 recursively copies the contents of src into dst.
func deriveDeepCopy_3(dst, src []DirectRELPSourceConfig) {
	for src_i, src_value := range src {
		field := new(DirectRELPSourceConfig)
		deriveDeepCopy_11(field, &src_value)
		dst[src_i] = *field
	}
}

// deriveDeepCopy_4 recursively copies the contents of src into dst.
func deriveDeepCopy_4(dst, src []KafkaSourceConfig) {
	for src_i, src_value := range src {
		field := new(KafkaSourceConfig)
		deriveDeepCopy_12(field, &src_value)
		dst[src_i] = *field
	}
}

// deriveDeepCopy_5 recursively copies the contents of src into dst.
func deriveDeepCopy_5(dst, src []GraylogSourceConfig) {
	for src_i, src_value := range src {
		field := new(GraylogSourceConfig)
		deriveDeepCopy_13(field, &src_value)
		dst[src_i] = *field
	}
}

// deriveDeepCopy_6 recursively copies the contents of src into dst.
func deriveDeepCopy_6(dst, src *KafkaDestConfig) {
	field := new(KafkaBaseConfig)
	deriveDeepCopy_14(field, &src.KafkaBaseConfig)
	dst.KafkaBaseConfig = *field
	dst.KafkaProducerBaseConfig = src.KafkaProducerBaseConfig
	dst.TlsBaseConfig = src.TlsBaseConfig
	dst.Insecure = src.Insecure
	dst.Format = src.Format
}

// deriveDeepCopy_7 recursively copies the contents of src into dst.
func deriveDeepCopy_7(dst, src *NATSDestConfig) {
	dst.TlsBaseConfig = src.TlsBaseConfig
	dst.Insecure = src.Insecure
	if src.NServers == nil {
		dst.NServers = nil
	} else {
		if dst.NServers != nil {
			if len(src.NServers) > len(dst.NServers) {
				if cap(dst.NServers) >= len(src.NServers) {
					dst.NServers = (dst.NServers)[:len(src.NServers)]
				} else {
					dst.NServers = make([]string, len(src.NServers))
				}
			} else if len(src.NServers) < len(dst.NServers) {
				dst.NServers = (dst.NServers)[:len(src.NServers)]
			}
		} else {
			dst.NServers = make([]string, len(src.NServers))
		}
		copy(dst.NServers, src.NServers)
	}
	dst.Format = src.Format
	dst.Name = src.Name
	dst.MaxReconnect = src.MaxReconnect
	dst.ReconnectWait = src.ReconnectWait
	dst.ConnTimeout = src.ConnTimeout
	dst.FlusherTimeout = src.FlusherTimeout
	dst.PingInterval = src.PingInterval
	dst.MaxPingsOut = src.MaxPingsOut
	dst.ReconnectBufSize = src.ReconnectBufSize
	dst.Username = src.Username
	dst.Password = src.Password
	dst.NoRandomize = src.NoRandomize
	dst.AllowReconnect = src.AllowReconnect
}

// deriveDeepCopy_8 recursively copies the contents of src into dst.
func deriveDeepCopy_8(dst, src *TCPSourceConfig) {
	field := new(SyslogSourceBaseConfig)
	deriveDeepCopy_15(field, &src.SyslogSourceBaseConfig)
	dst.SyslogSourceBaseConfig = *field
	dst.FilterSubConfig = src.FilterSubConfig
	dst.TlsBaseConfig = src.TlsBaseConfig
	dst.ClientAuthType = src.ClientAuthType
	dst.LineFraming = src.LineFraming
	dst.FrameDelimiter = src.FrameDelimiter
	dst.ConfID = src.ConfID
}

// deriveDeepCopy_9 recursively copies the contents of src into dst.
func deriveDeepCopy_9(dst, src *UDPSourceConfig) {
	field := new(SyslogSourceBaseConfig)
	deriveDeepCopy_15(field, &src.SyslogSourceBaseConfig)
	dst.SyslogSourceBaseConfig = *field
	dst.FilterSubConfig = src.FilterSubConfig
	dst.ConfID = src.ConfID
}

// deriveDeepCopy_10 recursively copies the contents of src into dst.
func deriveDeepCopy_10(dst, src *RELPSourceConfig) {
	field := new(SyslogSourceBaseConfig)
	deriveDeepCopy_15(field, &src.SyslogSourceBaseConfig)
	dst.SyslogSourceBaseConfig = *field
	dst.FilterSubConfig = src.FilterSubConfig
	dst.TlsBaseConfig = src.TlsBaseConfig
	dst.ClientAuthType = src.ClientAuthType
	dst.LineFraming = src.LineFraming
	dst.FrameDelimiter = src.FrameDelimiter
	dst.ConfID = src.ConfID
}

// deriveDeepCopy_11 recursively copies the contents of src into dst.
func deriveDeepCopy_11(dst, src *DirectRELPSourceConfig) {
	field := new(SyslogSourceBaseConfig)
	deriveDeepCopy_15(field, &src.SyslogSourceBaseConfig)
	dst.SyslogSourceBaseConfig = *field
	dst.FilterSubConfig = src.FilterSubConfig
	dst.TlsBaseConfig = src.TlsBaseConfig
	dst.ClientAuthType = src.ClientAuthType
	dst.LineFraming = src.LineFraming
	dst.FrameDelimiter = src.FrameDelimiter
	dst.ConfID = src.ConfID
}

// deriveDeepCopy_12 recursively copies the contents of src into dst.
func deriveDeepCopy_12(dst, src *KafkaSourceConfig) {
	field := new(KafkaBaseConfig)
	deriveDeepCopy_14(field, &src.KafkaBaseConfig)
	dst.KafkaBaseConfig = *field
	dst.KafkaConsumerBaseConfig = src.KafkaConsumerBaseConfig
	dst.FilterSubConfig = src.FilterSubConfig
	dst.TlsBaseConfig = src.TlsBaseConfig
	dst.Insecure = src.Insecure
	dst.Format = src.Format
	dst.Encoding = src.Encoding
	dst.ConfID = src.ConfID
	dst.SessionTimeout = src.SessionTimeout
	dst.HeartbeatInterval = src.HeartbeatInterval
	dst.OffsetsMaxRetry = src.OffsetsMaxRetry
	dst.GroupID = src.GroupID
	if src.Topics == nil {
		dst.Topics = nil
	} else {
		if dst.Topics != nil {
			if len(src.Topics) > len(dst.Topics) {
				if cap(dst.Topics) >= len(src.Topics) {
					dst.Topics = (dst.Topics)[:len(src.Topics)]
				} else {
					dst.Topics = make([]string, len(src.Topics))
				}
			} else if len(src.Topics) < len(dst.Topics) {
				dst.Topics = (dst.Topics)[:len(src.Topics)]
			}
		} else {
			dst.Topics = make([]string, len(src.Topics))
		}
		copy(dst.Topics, src.Topics)
	}
}

// deriveDeepCopy_13 recursively copies the contents of src into dst.
func deriveDeepCopy_13(dst, src *GraylogSourceConfig) {
	field := new(SyslogSourceBaseConfig)
	deriveDeepCopy_15(field, &src.SyslogSourceBaseConfig)
	dst.SyslogSourceBaseConfig = *field
	dst.FilterSubConfig = src.FilterSubConfig
	dst.ConfID = src.ConfID
}

// deriveDeepCopy_14 recursively copies the contents of src into dst.
func deriveDeepCopy_14(dst, src *KafkaBaseConfig) {
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
}

// deriveDeepCopy_15 recursively copies the contents of src into dst.
func deriveDeepCopy_15(dst, src *SyslogSourceBaseConfig) {
	if src.Ports == nil {
		dst.Ports = nil
	} else {
		if dst.Ports != nil {
			if len(src.Ports) > len(dst.Ports) {
				if cap(dst.Ports) >= len(src.Ports) {
					dst.Ports = (dst.Ports)[:len(src.Ports)]
				} else {
					dst.Ports = make([]int, len(src.Ports))
				}
			} else if len(src.Ports) < len(dst.Ports) {
				dst.Ports = (dst.Ports)[:len(src.Ports)]
			}
		} else {
			dst.Ports = make([]int, len(src.Ports))
		}
		copy(dst.Ports, src.Ports)
	}
	dst.BindAddr = src.BindAddr
	dst.UnixSocketPath = src.UnixSocketPath
	dst.Format = src.Format
	dst.KeepAlive = src.KeepAlive
	dst.KeepAlivePeriod = src.KeepAlivePeriod
	dst.Timeout = src.Timeout
	dst.Encoding = src.Encoding
}
