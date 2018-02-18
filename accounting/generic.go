package accounting

import "encoding/json"

func (a *Acct) Marshal() string {
	b, _ := json.Marshal(a)
	return string(b)
}
