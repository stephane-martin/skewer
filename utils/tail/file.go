package tail

import (
	"bytes"
	"io"
	"os"
)

func fileLines(file *os.File, nbLines int, startPos, endPos int64, output io.Writer) (readPos int64, err error) {
	if nbLines <= 0 {
		return
	}
	buf := make([]byte, bufferSize)
	pos := endPos
	bytesRead := int((pos - startPos) % bufferSize)
	if bytesRead == 0 {
		bytesRead = bufferSize
	}
	pos -= int64(bytesRead)

	file.Seek(pos, 0)
	bytesRead, err = io.ReadFull(file, buf[:bytesRead])
	readPos = pos + int64(bytesRead)
	if err != nil {
		return
	}

	if buf[bytesRead-1] != lineEnd {
		nbLines--
	}

	for bytesRead > 0 {
		n := bytesRead

		for n > 0 {
			n = bytes.LastIndex(buf[:n], lineEndS)
			if n == -1 {
				break
			}
			if nbLines == 0 {
				l := bytesRead - (n + 1)
				if l > 0 {
					output.Write(buf[n+1 : n+1+l])
				}
				l2 := int64(endPos - (pos + int64(bytesRead)))
				if l > 0 {
					var n int64
					n, err = io.CopyN(output, file, l2)
					readPos += n
				}
				return
			}
			nbLines--
		}

		if pos == startPos {
			file.Seek(startPos, 0)
			var n int64
			n, err = io.Copy(output, file)
			readPos += n
			return
		}
		pos -= bufferSize
		file.Seek(pos, 0)
		bytesRead, err = io.ReadFull(file, buf)
		readPos = pos + int64(bytesRead)
		if err != nil {
			return
		}
	}

	return
}
