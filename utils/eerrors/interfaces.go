package eerrors

type causes interface {
	Causes() []error
}

type cause interface {
	Cause() error
}

type types interface {
	Types() []string
}
