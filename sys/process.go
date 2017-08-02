package sys

import (
	"github.com/shirou/gopsutil/process"
)

func HasAnyProcess(names []string) bool {
	pids, err := process.Pids()
	if err != nil {
		// well... can't do better in that case
		return false
	}

	m := map[string]bool{}
	for _, name := range names {
		m[name] = true
	}

	for _, pid := range pids {
		p, err := process.NewProcess(pid)
		if err == nil {
			name, err := p.Name()
			if err == nil {
				if m[name] {
					return true
				}
			}
		}
	}
	return false
}
