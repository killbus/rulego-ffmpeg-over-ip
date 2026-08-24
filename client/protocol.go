// Portions of this file are adapted from steelbrain/ffmpeg-over-ip v5.2.1
// (commit ab7adfeedf2a50f7e5807beef9088609cce645d6), under the MIT license.
package client

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

const (
	ProtocolVersion = uint8(0x06)
	MaxPayloadLen   = 100 * 1024 * 1024
	MaxFileReadLen  = MaxPayloadLen - 2

	msgCommand  = uint8(0x01)
	msgCancel   = uint8(0x02)
	msgExitCode = uint8(0x03)
	msgError    = uint8(0x04)
	msgPing     = uint8(0x05)
	msgPong     = uint8(0x06)

	msgStdin      = uint8(0x10)
	msgStdinClose = uint8(0x11)
	msgStdout     = uint8(0x12)
	msgStderr     = uint8(0x13)

	msgOpen      = uint8(0x20)
	msgRead      = uint8(0x21)
	msgWrite     = uint8(0x22)
	msgSeek      = uint8(0x23)
	msgClose     = uint8(0x24)
	msgFstat     = uint8(0x25)
	msgFtruncate = uint8(0x26)
	msgUnlink    = uint8(0x27)
	msgRename    = uint8(0x28)
	msgMkdir     = uint8(0x29)

	msgOpenOK      = uint8(0x40)
	msgReadOK      = uint8(0x41)
	msgWriteOK     = uint8(0x42)
	msgSeekOK      = uint8(0x43)
	msgCloseOK     = uint8(0x44)
	msgFstatOK     = uint8(0x45)
	msgFtruncateOK = uint8(0x46)
	msgUnlinkOK    = uint8(0x47)
	msgRenameOK    = uint8(0x48)
	msgMkdirOK     = uint8(0x49)
	msgIOError     = uint8(0x4f)

	programFFmpeg  = uint8(0x01)
	programFFprobe = uint8(0x02)

	nonceLen = 16
	hmacLen  = 32
)

type frame struct {
	typ     uint8
	payload []byte
}

func programID(program string) (uint8, error) {
	switch program {
	case "ffmpeg":
		return programFFmpeg, nil
	case "ffprobe":
		return programFFprobe, nil
	default:
		return 0, fmt.Errorf("program must be ffmpeg or ffprobe")
	}
}

// ValidateInvocation checks every value before command encoding can narrow it
// to the protocol's unsigned 16-bit fields.
func ValidateInvocation(program string, args []string) error {
	_, _, err := validateCommand(program, args)
	return err
}

func validateCommand(program string, args []string) (uint8, int, error) {
	programByte, err := programID(program)
	if err != nil {
		return 0, 0, err
	}
	if len(args) > math.MaxUint16 {
		return 0, 0, fmt.Errorf("too many arguments")
	}
	size := 1 + nonceLen + hmacLen + 1 + 2
	for _, arg := range args {
		if len(arg) > math.MaxUint16 {
			return 0, 0, fmt.Errorf("argument is too long")
		}
		size += 2 + len(arg)
		if size > MaxPayloadLen {
			return 0, 0, fmt.Errorf("command is too large")
		}
	}
	return programByte, size, nil
}

func commandPayload(secret, program string, args []string, nonce [nonceLen]byte) ([]byte, error) {
	programByte, size, err := validateCommand(program, args)
	if err != nil {
		return nil, err
	}

	signature := signCommand(secret, programByte, args, nonce)
	payload := make([]byte, size)
	payload[0] = ProtocolVersion
	copy(payload[1:], nonce[:])
	copy(payload[1+nonceLen:], signature[:])
	payload[1+nonceLen+hmacLen] = programByte
	offset := 1 + nonceLen + hmacLen + 1
	binary.BigEndian.PutUint16(payload[offset:], uint16(len(args)))
	offset += 2
	for _, arg := range args {
		binary.BigEndian.PutUint16(payload[offset:], uint16(len(arg)))
		offset += 2
		copy(payload[offset:], arg)
		offset += len(arg)
	}
	return payload, nil
}

func signCommand(secret string, program uint8, args []string, nonce [nonceLen]byte) [hmacLen]byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte{ProtocolVersion})
	mac.Write(nonce[:])
	mac.Write([]byte{program})
	var word [2]byte
	binary.BigEndian.PutUint16(word[:], uint16(len(args)))
	mac.Write(word[:])
	for _, arg := range args {
		binary.BigEndian.PutUint16(word[:], uint16(len(arg)))
		mac.Write(word[:])
		mac.Write([]byte(arg))
	}
	var signature [hmacLen]byte
	copy(signature[:], mac.Sum(nil))
	return signature
}

func readFrame(r io.Reader) (frame, error) {
	var header [5]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return frame{}, err
	}
	length := binary.BigEndian.Uint32(header[1:])
	if length > MaxPayloadLen {
		return frame{}, fmt.Errorf("frame exceeds protocol limit")
	}
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(r, payload); err != nil {
		return frame{}, err
	}
	return frame{typ: header[0], payload: payload}, nil
}

func writeFrame(w io.Writer, typ uint8, payload []byte) error {
	if len(payload) > MaxPayloadLen {
		return fmt.Errorf("frame exceeds protocol limit")
	}
	var header [5]byte
	header[0] = typ
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	if err := writeAll(w, header[:]); err != nil {
		return err
	}
	return writeAll(w, payload)
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
