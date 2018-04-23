package conf

import (
	"github.com/stephane-martin/skewer/utils/eerrors"
)

func configurationError(err error) error {
	return eerrors.WithTypes(err, "Configuration")
}

func confReadError(err error, filename string) error {
	return configurationError(
		eerrors.WithTags(
			eerrors.Wrap(err, "Error reading configuration file"),
			"filename", filename,
		),
	)
}

func confSyntaxError(err error, filename string) error {
	return configurationError(
		eerrors.WithTags(
			eerrors.Wrap(err, "Syntax error in configuration file"),
			"filename", filename,
		),
	)
}

func confCheckError(err error) error {
	return configurationError(
		eerrors.Wrap(err, "Configuration check failed"),
	)
}
