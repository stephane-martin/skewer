package namespaces

import (
	"os/exec"
	"strings"
)

type NamespacedCmd struct {
	cmd          *exec.Cmd
	dumpable     bool
	storePath    string
	confPath     string
	acctPath     string
	fileDestTmpl string
	certFiles    []string
	certPaths    []string
}

func NewNamespacedCmd(cmd *exec.Cmd) *NamespacedCmd {
	return &NamespacedCmd{cmd: cmd}
}

func (c *NamespacedCmd) Dumpable(dumpable bool) *NamespacedCmd {
	c.dumpable = dumpable
	return c
}

func (c *NamespacedCmd) StorePath(path string) *NamespacedCmd {
	c.storePath = strings.TrimSpace(path)
	return c
}

func (c *NamespacedCmd) ConfPath(path string) *NamespacedCmd {
	c.confPath = strings.TrimSpace(path)
	return c
}

func (c *NamespacedCmd) AccountingPath(path string) *NamespacedCmd {
	c.acctPath = strings.TrimSpace(path)
	return c
}

func (c *NamespacedCmd) FileDestTemplate(tmpl string) *NamespacedCmd {
	c.fileDestTmpl = strings.TrimSpace(tmpl)
	return c
}


