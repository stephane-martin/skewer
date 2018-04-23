package javascript

import (
	"github.com/dop251/goja"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

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

func jsvmError(err error) error {
	return eerrors.WithTypes(err, "Javascript")
}

func objectNotFoundError(obj string) error {
	return jsvmError(eerrors.Errorf("Object was not found in the JS VM: '%s'", obj))
}

func notAFunctionError(obj string) error {
	return jsvmError(eerrors.Errorf("The object is not a JS function: '%s'", obj))
}

func jsExceptionError(exc *goja.Exception, funcname string) error {
	return jsvmError(
		eerrors.Errorf(
			"A JS exception happened when executing the JS function '%s': %s",
			funcname, exc.String(),
		),
	)
}

func executingJSError(err error, funcname string) error {
	return jsvmError(eerrors.Wrapf(err, "An unexpected error happened when executing the JS function '%s'", funcname))
}

func executingJSErrorFactory(err error, funcname string) error {
	if jsexc, ok := err.(*goja.Exception); ok {
		return jsExceptionError(jsexc, funcname)
	}
	return executingJSError(err, funcname)
}

func go2jsError(err error) error {
	return jsvmError(eerrors.Wrap(err, "Error converting a Go variable to JS"))
}

func js2goError(err error) error {
	return jsvmError(eerrors.Wrap(err, "Error converting a JS variable to Go"))
}

func jsDecodingError(msg string, decoderName string) error {
	return eerrors.WithTypes(
		eerrors.Errorf(
			"The provided JS parser '%s' could not parse the following message: %s",
			decoderName, msg,
		),
		"Decoding", "Javascript",
	)
}
