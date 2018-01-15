package tail

import (
	"context"
	"strings"
)

// TODO: use wg instead of ctx ?
func removeNLChans(ctx context.Context, input chan string, output chan string) {
	go func() {
		defer func() {
			if output != nil {
				close(output)
			}
		}()
		var line string
		var more bool
		for {
			select {
			case <-ctx.Done():
				return
			case line, more = <-input:
				if more {
					if output != nil {
						output <- strings.Trim(line, lineEndString)
					}
				} else {
					return
				}
			}
		}
	}()
}
