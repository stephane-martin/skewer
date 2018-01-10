package tail

import (
	"io"
	"os"
	"sync"
	"time"
)

func followClassical(cancelChan <-chan struct{}, wg *sync.WaitGroup, fspec *fileSpec, sleepInterval time.Duration) {
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		blockingStdin := false
		if fspec.file != nil && fspec.name == "-" {
			infos, err := fspec.file.Stat()
			if err == nil {
				if !isRegular(infos.Mode()) {
					blockingStdin = true
				}
			} else if fspec.errors != nil {
				fspec.errors <- err
			}
		}
	Loop:
		for {
			if fspec.ignore {
				return
			}

			havenew, err := step(fspec, blockingStdin)

			if err == io.EOF {
				return
			} else if err != nil && fspec.errors != nil {
				fspec.errors <- err
			}

			if havenew {
				select {
				case <-cancelChan:
					return
				default:
					continue Loop
				}
			}

			select {
			case <-cancelChan:
				return
			case <-time.After(sleepInterval):
				continue Loop
			}
		}
	}()
}

func step(fspec *fileSpec, blockingStdin bool) (bool, error) {
	if fspec.file == nil {
		fspec.recheck(false)
		return false, nil
	}

	mode := fspec.mode
	var infos os.FileInfo
	var err error

	if !blockingStdin {
		infos, err = fspec.file.Stat()
		if err != nil {
			fspec.err = err
			fspec.file.Close()
			fspec.file = nil
			return false, err
		}

		if infos.ModTime() == fspec.mtime && infos.Mode() == fspec.mode && (!isRegular(fspec.mode) || infos.Size() == fspec.size) {
			if fspec.unchangedStats >= maxUnchangedStat {
				fspec.recheck(false)
				fspec.unchangedStats = 0
			} else {
				fspec.unchangedStats++
			}
			return false, nil
		}

		fspec.mtime = infos.ModTime()
		fspec.mode = infos.Mode()
		fspec.unchangedStats = 0

		if isRegular(mode) && fspec.size > infos.Size() {
			// truncated
			fspec.file.Seek(0, 0)
			fspec.size = 0
		}
	}

	var nread int64
	if blockingStdin {
		nread, err = io.CopyN(fspec.writer, fspec.file, bufferSize)
	} else if isRegular(mode) && fspec.remote {
		nread, err = io.CopyN(fspec.writer, fspec.file, infos.Size()-fspec.size)
		if err == io.EOF {
			err = nil
		}
	} else {
		nread, err = io.Copy(fspec.writer, fspec.file)
		if err == io.EOF {
			err = nil
		}
	}
	fspec.size += nread
	return nread > 0, err
}
