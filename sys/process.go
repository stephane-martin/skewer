package sys

//go:generate goderive .

import (
	"github.com/shirou/gopsutil/process"
)

func HasAnyProcess(wanted []string) bool {
	return len(deriveIntersectNames(deriveUniqueNames(wanted), ListProcessNames())) > 0
}

func ListProcessNames() (names []string) {
	names = []string{}
	pids, err := process.Pids()
	if err != nil {
		return nil
	}
	for _, pid := range pids {
		p, err := process.NewProcess(pid)
		if err == nil {
			name, err := p.Name()
			if err == nil {
				names = append(names, name)
			}
		}
	}
	return deriveUniqueNames(names)
}
