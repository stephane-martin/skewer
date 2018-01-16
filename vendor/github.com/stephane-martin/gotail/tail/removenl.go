package tail

import (
	"bytes"
	"context"
)

// TODO: use wg instead of ctx ?
func removeNLChans(ctx context.Context, input chan []byte, output chan []byte) {
	go func() {
		defer func() {
			if output != nil {
				close(output)
			}
		}()
		var line []byte
		var more bool
		for {
			select {
			case <-ctx.Done():
				return
			case line, more = <-input:
				if more {
					if output != nil {
						output <- bytes.Trim(line, lineEndString)
					}
				} else {
					return
				}
			}
		}
	}()
}
