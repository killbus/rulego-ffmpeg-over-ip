// Portions of this file are adapted from steelbrain/ffmpeg-over-ip v5.2.1
// (commit ab7adfeedf2a50f7e5807beef9088609cce645d6), under the MIT license.
package client

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

const (
	fioEPERM   = int32(1)
	fioENOENT  = int32(2)
	fioEIO     = int32(5)
	fioEACCES  = int32(13)
	fioEEXIST  = int32(17)
	fioENOTDIR = int32(20)
	fioEISDIR  = int32(21)
	fioEINVAL  = int32(22)
	fioENOSPC  = int32(28)
	fioEROFS   = int32(30)
	fioERANGE  = int32(34)
)

type fileHandler struct {
	files       map[uint16]*os.File
	written     map[uint16]string
	ready       map[string]struct{}
	onFileReady func(string)
	readBuf     []byte
}

func newFileHandler(onFileReady func(string)) *fileHandler {
	return &fileHandler{
		files:       make(map[uint16]*os.File),
		written:     make(map[uint16]string),
		ready:       make(map[string]struct{}),
		onFileReady: onFileReady,
	}
}

func (h *fileHandler) closeAll() {
	for id, file := range h.files {
		_ = file.Close()
		delete(h.files, id)
	}
}

func (h *fileHandler) handle(typ uint8, payload []byte, write func(uint8, []byte) error) error {
	switch typ {
	case msgOpen:
		return h.open(payload, write)
	case msgRead:
		return h.read(payload, write)
	case msgWrite:
		return h.write(payload, write)
	case msgSeek:
		return h.seek(payload, write)
	case msgClose:
		return h.close(payload, write)
	case msgFstat:
		return h.fstat(payload, write)
	case msgFtruncate:
		return h.truncate(payload, write)
	case msgUnlink:
		return h.unlink(payload, write)
	case msgRename:
		return h.rename(payload, write)
	case msgMkdir:
		return h.mkdir(payload, write)
	default:
		return fmt.Errorf("unknown file request")
	}
}

func (h *fileHandler) open(p []byte, write func(uint8, []byte) error) error {
	if len(p) < 10 {
		return fmt.Errorf("malformed open request")
	}
	req, id := binary.BigEndian.Uint16(p), binary.BigEndian.Uint16(p[2:])
	if _, exists := h.files[id]; exists {
		return writeIOError(write, req, fioEINVAL)
	}
	wireFlag := binary.BigEndian.Uint32(p[4:])
	flags := wireFlags(wireFlag)
	path := string(p[10:])
	if binary.BigEndian.Uint32(p[4:])&0x40 != 0 {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return writeIOError(write, req, mapErrno(err))
		}
	}
	file, err := os.OpenFile(path, flags, os.FileMode(binary.BigEndian.Uint16(p[8:])))
	if err != nil {
		return writeIOError(write, req, mapErrno(err))
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return writeIOError(write, req, mapErrno(err))
	}
	h.files[id] = file
	if wireFlag&3 != 0 || wireFlag&0x40 != 0 || wireFlag&0x200 != 0 {
		h.written[id] = path
	}
	response := make([]byte, 10)
	binary.BigEndian.PutUint16(response, req)
	binary.BigEndian.PutUint64(response[2:], uint64(info.Size()))
	return write(msgOpenOK, response)
}

func (h *fileHandler) read(p []byte, write func(uint8, []byte) error) error {
	if len(p) < 8 {
		return fmt.Errorf("malformed read request")
	}
	req, id := binary.BigEndian.Uint16(p), binary.BigEndian.Uint16(p[2:])
	n := binary.BigEndian.Uint32(p[4:])
	if n > MaxFileReadLen {
		return writeIOError(write, req, fioERANGE)
	}
	file, ok := h.files[id]
	if !ok {
		return writeIOError(write, req, fioEINVAL)
	}
	length := 2 + int(n)
	if cap(h.readBuf) < length {
		h.readBuf = make([]byte, length)
	}
	response := h.readBuf[:length]
	binary.BigEndian.PutUint16(response, req)
	read, err := file.Read(response[2:])
	if err != nil && err != io.EOF {
		return writeIOError(write, req, mapErrno(err))
	}
	return write(msgReadOK, response[:2+read])
}

func (h *fileHandler) write(p []byte, send func(uint8, []byte) error) error {
	if len(p) < 4 {
		return fmt.Errorf("malformed write request")
	}
	req, id := binary.BigEndian.Uint16(p), binary.BigEndian.Uint16(p[2:])
	file, ok := h.files[id]
	if !ok {
		return writeIOError(send, req, fioEINVAL)
	}
	n, err := file.Write(p[4:])
	if err != nil {
		return writeIOError(send, req, mapErrno(err))
	}
	response := make([]byte, 6)
	binary.BigEndian.PutUint16(response, req)
	binary.BigEndian.PutUint32(response[2:], uint32(n))
	return send(msgWriteOK, response)
}

func (h *fileHandler) seek(p []byte, write func(uint8, []byte) error) error {
	if len(p) < 13 {
		return fmt.Errorf("malformed seek request")
	}
	req, id := binary.BigEndian.Uint16(p), binary.BigEndian.Uint16(p[2:])
	file, ok := h.files[id]
	if !ok {
		return writeIOError(write, req, fioEINVAL)
	}
	whence := -1
	switch p[12] {
	case 0:
		whence = io.SeekStart
	case 1:
		whence = io.SeekCurrent
	case 2:
		whence = io.SeekEnd
	}
	if whence < 0 {
		return writeIOError(write, req, fioEINVAL)
	}
	offset, err := file.Seek(int64(binary.BigEndian.Uint64(p[4:])), whence)
	if err != nil {
		return writeIOError(write, req, mapErrno(err))
	}
	response := make([]byte, 10)
	binary.BigEndian.PutUint16(response, req)
	binary.BigEndian.PutUint64(response[2:], uint64(offset))
	return write(msgSeekOK, response)
}

func (h *fileHandler) close(p []byte, write func(uint8, []byte) error) error {
	if len(p) < 4 {
		return fmt.Errorf("malformed close request")
	}
	req, id := binary.BigEndian.Uint16(p), binary.BigEndian.Uint16(p[2:])
	file, ok := h.files[id]
	if !ok {
		return writeIOError(write, req, fioEINVAL)
	}
	err := file.Close()
	delete(h.files, id)
	path, written := h.written[id]
	delete(h.written, id)
	if err != nil {
		return writeIOError(write, req, mapErrno(err))
	}
	if written {
		h.markReady(path)
	}
	return writeRequestOK(write, msgCloseOK, req)
}

func (h *fileHandler) fstat(p []byte, write func(uint8, []byte) error) error {
	if len(p) < 4 {
		return fmt.Errorf("malformed fstat request")
	}
	req, id := binary.BigEndian.Uint16(p), binary.BigEndian.Uint16(p[2:])
	file, ok := h.files[id]
	if !ok {
		return writeIOError(write, req, fioEINVAL)
	}
	info, err := file.Stat()
	if err != nil {
		return writeIOError(write, req, mapErrno(err))
	}
	response := make([]byte, 14)
	binary.BigEndian.PutUint16(response, req)
	binary.BigEndian.PutUint64(response[2:], uint64(info.Size()))
	binary.BigEndian.PutUint32(response[10:], uint32(info.Mode()))
	return write(msgFstatOK, response)
}

func (h *fileHandler) truncate(p []byte, write func(uint8, []byte) error) error {
	if len(p) < 12 {
		return fmt.Errorf("malformed truncate request")
	}
	req, id := binary.BigEndian.Uint16(p), binary.BigEndian.Uint16(p[2:])
	file, ok := h.files[id]
	if !ok {
		return writeIOError(write, req, fioEINVAL)
	}
	if err := file.Truncate(int64(binary.BigEndian.Uint64(p[4:]))); err != nil {
		return writeIOError(write, req, mapErrno(err))
	}
	return writeRequestOK(write, msgFtruncateOK, req)
}

func (h *fileHandler) unlink(p []byte, write func(uint8, []byte) error) error {
	if len(p) < 2 {
		return fmt.Errorf("malformed unlink request")
	}
	req := binary.BigEndian.Uint16(p)
	if err := os.Remove(string(p[2:])); err != nil {
		return writeIOError(write, req, mapErrno(err))
	}
	return writeRequestOK(write, msgUnlinkOK, req)
}

func (h *fileHandler) rename(p []byte, write func(uint8, []byte) error) error {
	if len(p) < 4 {
		return fmt.Errorf("malformed rename request")
	}
	req, oldLen := binary.BigEndian.Uint16(p), int(binary.BigEndian.Uint16(p[2:]))
	if len(p) < 4+oldLen {
		return fmt.Errorf("malformed rename request")
	}
	oldPath, newPath := string(p[4:4+oldLen]), string(p[4+oldLen:])
	if err := os.Rename(oldPath, newPath); err != nil {
		return writeIOError(write, req, mapErrno(err))
	}
	if _, ok := h.ready[oldPath]; ok {
		delete(h.ready, oldPath)
		h.markReady(newPath)
	}
	return writeRequestOK(write, msgRenameOK, req)
}

func (h *fileHandler) markReady(path string) {
	h.ready[path] = struct{}{}
	if h.onFileReady != nil {
		h.onFileReady(path)
	}
}

func (h *fileHandler) mkdir(p []byte, write func(uint8, []byte) error) error {
	if len(p) < 4 {
		return fmt.Errorf("malformed mkdir request")
	}
	req := binary.BigEndian.Uint16(p)
	if err := os.Mkdir(string(p[4:]), os.FileMode(binary.BigEndian.Uint16(p[2:]))); err != nil {
		return writeIOError(write, req, mapErrno(err))
	}
	return writeRequestOK(write, msgMkdirOK, req)
}

func wireFlags(flags uint32) int {
	var result int
	switch flags & 3 {
	case 1:
		result = os.O_WRONLY
	case 2:
		result = os.O_RDWR
	}
	if flags&0x40 != 0 {
		result |= os.O_CREATE
	}
	if flags&0x200 != 0 {
		result |= os.O_TRUNC
	}
	return result
}

func writeRequestOK(write func(uint8, []byte) error, typ uint8, requestID uint16) error {
	payload := make([]byte, 2)
	binary.BigEndian.PutUint16(payload, requestID)
	return write(typ, payload)
}

func writeIOError(write func(uint8, []byte) error, requestID uint16, errno int32) error {
	payload := make([]byte, 6)
	binary.BigEndian.PutUint16(payload, requestID)
	binary.BigEndian.PutUint32(payload[2:], uint32(errno))
	return write(msgIOError, payload)
}

func mapErrno(err error) int32 {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.EPERM:
			return fioEPERM
		case syscall.ENOENT:
			return fioENOENT
		case syscall.EIO:
			return fioEIO
		case syscall.EACCES:
			return fioEACCES
		case syscall.EEXIST:
			return fioEEXIST
		case syscall.ENOTDIR:
			return fioENOTDIR
		case syscall.EISDIR:
			return fioEISDIR
		case syscall.EINVAL:
			return fioEINVAL
		case syscall.ENOSPC:
			return fioENOSPC
		case syscall.EROFS:
			return fioEROFS
		case syscall.ERANGE:
			return fioERANGE
		}
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fioENOENT
	case errors.Is(err, fs.ErrExist):
		return fioEEXIST
	case errors.Is(err, fs.ErrPermission):
		return fioEACCES
	case errors.Is(err, fs.ErrInvalid):
		return fioEINVAL
	default:
		return fioEIO
	}
}
