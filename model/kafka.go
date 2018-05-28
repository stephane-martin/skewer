package model

import sarama "github.com/Shopify/sarama"

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
		sarama.ErrPolicyViolation,
		sarama.ErrConsumerCoordinatorNotAvailable:
		return true
	default:
		return false
	}
}
