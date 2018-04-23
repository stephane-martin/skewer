package decoders

import "github.com/stephane-martin/skewer/utils/eerrors"

func ErrorUnknownFormat(format string) error {
	return DecodingError(eerrors.Errorf("Unknown decoder: '%s'", format))
}

func InvalidTopicError(topic string) error {
	return DecodingError(
		eerrors.Errorf("The topic name is invalid: '%s'", topic),
	)
}

func DecodingError(err error) error {
	return eerrors.WithTypes(
		eerrors.Wrap(err, "Error decoding message"),
		"Decoding",
	)
}

var EmptyMessageError = DecodingError(eerrors.New("Empty message"))

func UnmarshalJsonError(err error) error {
	return DecodingError(
		eerrors.Wrap(err, "Error unmarshaling the message from JSON"),
	)
}

var ErrInvalidTimestamp = DecodingError(eerrors.New("TimeReported and TimeGenerated should be formatted in RFC3339 format"))

func InvalidCharsetError(err error) error {
	return DecodingError(
		eerrors.Wrap(err, "The input message was not properly encoded with specified charset"),
	)
}

func InfluxDecodingError(err error) error {
	return DecodingError(
		eerrors.Wrap(err, "Error decoding InfluxDB line protocol"),
	)
}

func RFC5424DecodingError(err error) error {
	return DecodingError(
		eerrors.Wrap(err, "Error decoding RFC5424 message"),
	)
}

func W3CDecodingError(err error) error {
	return DecodingError(
		eerrors.Wrap(err, "Error decoding Extended Log Format message"),
	)
}

var ErrInvalidSD = DecodingError(eerrors.New("Invalid structured data"))

var ErrInvalidPriority = DecodingError(eerrors.New("Invalid priority field"))

func NotEnoughPartsError(nb int) error {
	return DecodingError(
		eerrors.Errorf("The message does not have enough parts: %d, but minimum is 7", nb),
	)
}
