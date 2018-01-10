package tail

import "os"

const bufferSize = 512 * 8

var lineEnd byte = '\n'
var lineEndS = []byte{lineEnd}
var lineEndString = "\n"
var ss = struct{}{}

func isCharDevice(mode os.FileMode) bool {
	return (mode & os.ModeCharDevice) != 0
}

func isRegular(mode os.FileMode) bool {
	return mode.IsRegular()
}

func isFIFO(mode os.FileMode) bool {
	return (mode & os.ModeNamedPipe) != 0
}

func isSocket(mode os.FileMode) bool {
	return (mode & os.ModeSocket) != 0
}

func isLink(mode os.FileMode) bool {
	return (mode & os.ModeSymlink) != 0
}

func isDevice(mode os.FileMode) bool {
	return (mode & os.ModeDevice) != 0
}

func isTailable(mode os.FileMode) bool {
	return isRegular(mode) || isFIFO(mode) || isCharDevice(mode) || isSocket(mode)
}
