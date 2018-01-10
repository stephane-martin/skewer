package tail

import (
	"bytes"
	"io"
	"os"
)

type lineBuffer struct {
	buffer [bufferSize]byte
	nBytes int
	nLines int
	next   *lineBuffer
}

func pipeLines(file *os.File, nbLines int, output io.Writer) (readPos int64, err error) {
	if nbLines <= 0 {
		return
	}
	var totalLines int
	var nRead int

	first := &lineBuffer{}
	tmp := &lineBuffer{}
	last := first

	for {
		nRead, err = file.Read(tmp.buffer[:])
		readPos += int64(nRead)
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}
		tmp.nBytes = nRead
		tmp.nLines = bytes.Count(tmp.buffer[:tmp.nBytes], lineEndS)
		tmp.next = nil
		totalLines += tmp.nLines

		if (tmp.nBytes + last.nBytes) < bufferSize {
			copy(last.buffer[last.nBytes:], tmp.buffer[:tmp.nBytes])
			last.nBytes += tmp.nBytes
			last.nLines += tmp.nLines
		} else {
			last.next = tmp
			last = tmp
			if (totalLines - first.nLines) > nbLines {
				tmp = first
				totalLines -= first.nLines
				first = first.next
			} else {
				tmp = &lineBuffer{}
			}
		}
	}
	// when nbLines == 0, we exhaust the pipe then return
	if last.nBytes == 0 || nbLines == 0 {
		return
	}

	if last.buffer[last.nBytes-1] != lineEnd {
		last.nLines++
		totalLines++
	}

	tmp = first
	for (totalLines - tmp.nLines) > nbLines {
		totalLines -= tmp.nLines
		tmp = tmp.next
	}
	beg := tmp.buffer[:tmp.nBytes]
	j := totalLines - nbLines
	// We made sure that 'totalLines - nbLines <= tmp.nLines'
	for j > 0 {
		idx := bytes.Index(beg, lineEndS)
		if idx == -1 {
			panic("should not happen")
		}
		beg = beg[idx+1:]
		j--
	}
	output.Write(beg)

	tmp = tmp.next
	for tmp != nil {
		output.Write(tmp.buffer[:tmp.nBytes])
		tmp = tmp.next
	}
	return
}
