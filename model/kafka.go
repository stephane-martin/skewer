package model

import sarama "gopkg.in/Shopify/sarama.v1"

func IsFatalKafkaError(e error) bool {
	switch e {
	case sarama.ErrOutOfBrokers,
		sarama.ErrUnknown,
		sarama.ErrUnknownTopicOrPartition,
		sarama.ErrReplicaNotAvailable,
		sarama.ErrLeaderNotAvailable,
		sarama.ErrNotLeaderForPartition,
		sarama.ErrNetworkException,
		sarama.ErrOffsetsLoadInProgress,
		sarama.ErrInvalidTopic,
		sarama.ErrNotEnoughReplicas,
		sarama.ErrNotEnoughReplicasAfterAppend,
		sarama.ErrTopicAuthorizationFailed,
		sarama.ErrGroupAuthorizationFailed,
		sarama.ErrClusterAuthorizationFailed,
		sarama.ErrUnsupportedVersion,
		sarama.ErrUnsupportedForMessageFormat,
		sarama.ErrPolicyViolation:
		return true
	default:
		return false
	}
}
