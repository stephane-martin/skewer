// +build linux

package auditlogs

//go:generate goderive .

import (
	"context"
	"syscall"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

var Supported bool = true

func WriteAuditLogs(ctx context.Context, c conf.AuditConfig, logger log15.Logger) (<-chan *model.AuditMessageGroup, error) {
	// canceling the context will make the NetlinkClient to be closed, and
	// client.Receive will return an error EBADF
	client, err := NewNetlinkClient(ctx, c.SocketBuffer)
	if err != nil {
		return nil, err
	}

	logger = logger.New("class", "audit")

	// buffered chan, in case of audit messages bursts
	netlinkMsgChan := make(chan *syscall.NetlinkMessage, 1000)
	interChan := make(chan *AuditMessageGroup, 1000)

	resultsChan := deriveFmapResults(copyMsg, interChan) // deep-copy messages from interChan to resultsChan

	go receive(client, netlinkMsgChan, logger)

	go consume(netlinkMsgChan, interChan, c, logger)

	return resultsChan, nil
}

func copyMsg(src *AuditMessageGroup) (cop *model.AuditMessageGroup) {
	cop = &model.AuditMessageGroup{AuditTime: src.AuditTime, Seq: src.Seq, UidMap: src.UidMap}
	if len(src.Msgs) > 0 {
		cop.Msgs = make([]*model.AuditSubMessage, 0, len(src.Msgs))
		for _, subMsg := range src.Msgs {
			cop.Msgs = append(cop.Msgs, &model.AuditSubMessage{Data: subMsg.Data, Type: subMsg.Type})
		}
	}
	return cop
}

func consume(source chan *syscall.NetlinkMessage, dest chan *AuditMessageGroup, c conf.AuditConfig, logger log15.Logger) {
	// consume is not transforming messages one to one: many netlink messages are merged into a single audit message
	marshaller := NewAuditMarshaller(
		dest,
		uint16(c.EventsMin),
		uint16(c.EventsMax),
		c.MessageTracking,
		c.LogOutOfOrder,
		c.MaxOutOfOrder,
		[]AuditFilter{},
		logger,
	)

	for msg := range source {
		marshaller.Consume(msg)
	}
	close(dest)
}

func receive(client *NetlinkClient, ch chan *syscall.NetlinkMessage, logger log15.Logger) {
	var err error
	var msg *syscall.NetlinkMessage
	for {
		msg, err = client.Receive()
		if err != nil {
			if err == syscall.EBADF {
				// happens when the context is canceled
				logger.Debug("The audit Netlink returned EBADF")
				close(ch)
				return
			} else if err == syscall.ENOTSOCK {
				logger.Error("The audit Netlink returned ENOTSOCK")
				close(ch)
				return
			} else {
				logger.Warn("Unknown error when receiving from the audit Netlink", "error", err)
			}
		} else if msg != nil {
			ch <- msg
		}
	}
}
