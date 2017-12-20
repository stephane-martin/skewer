// +build !linux

package namespaces

func PivotRoot(root string) (err error) {
	return nil
}

func SetJournalFs(targetExec string) error {
	return nil
}

func MakeChroot(targetExec string) (string, error) {
	return "", nil
}

func (c *NamespacedCmd) Start() error {
	return c.cmd.Start()
}
