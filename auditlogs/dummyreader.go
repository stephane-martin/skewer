// +build !linux

package auditlogs

import (
	"context"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

var Supported bool = false

func WriteAuditLogs(ctx context.Context, c conf.AuditConfig, logger log15.Logger) (chan *model.AuditMessageGroup, error) {
	resultsChan := make(chan *model.AuditMessageGroup)

	go func() {
		<-ctx.Done()
		close(resultsChan)
	}()

	return resultsChan, nil
}
