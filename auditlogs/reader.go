// +build linux

package auditlogs

import (
	"context"
	"syscall"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

var Supported bool = true

func WriteAuditLogs(ctx context.Context, c conf.AuditConfig, logger log15.Logger) (chan *model.AuditMessageGroup, error) {
	// canceling the context will make the NetlinkClient to be closed, and
	// client.Receive will return an error EBADF
	client, err := NewNetlinkClient(ctx, c.SocketBuffer)
	if err != nil {
		return nil, err
	}

	logger = logger.New("class", "audit")

	// buffered chan, in case of audit messages bursts
	netlinkMsgChan := make(chan *syscall.NetlinkMessage, 1000)
	interChan := make(chan *AuditMessageGroup)
	resultsChan := make(chan *model.AuditMessageGroup)

	marshaller := NewAuditMarshaller(
		interChan,
		uint16(c.EventsMin),
		uint16(c.EventsMax),
		c.MessageTracking,
		c.LogOutOfOrder,
		c.MaxOutOfOrder,
		[]AuditFilter{},
		logger,
	)

	go func() {
		for interMsg := range interChan {
			msg := &model.AuditMessageGroup{AuditTime: interMsg.AuditTime, Seq: interMsg.Seq, UidMap: interMsg.UidMap}
			if len(interMsg.Msgs) > 0 {
				msg.Msgs = make([]*model.AuditSubMessage, 0, len(interMsg.Msgs))
				for _, subMsg := range interMsg.Msgs {
					msg.Msgs = append(msg.Msgs, &model.AuditSubMessage{Data: subMsg.Data, Type: subMsg.Type})
				}
			}
			resultsChan <- msg
		}
		close(resultsChan)
	}()

	go func() {
		for {
			msg, err := client.Receive()
			if err != nil {
				if err == syscall.EBADF {
					logger.Debug("The audit Netlink returned EBADF")
					break
				} else {
					logger.Warn("Error when receiving from the audit Netlink", "error", err)
				}
			} else if msg != nil {
				netlinkMsgChan <- msg
			}
		}
		close(netlinkMsgChan)
	}()

	go func() {
		for msg := range netlinkMsgChan {
			marshaller.Consume(msg)
		}
		close(interChan)
	}()

	return resultsChan, nil
}
