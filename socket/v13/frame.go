package v13

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
)

const (
	OpContinuation = 0
	OpText         = 1
	OpBinary       = 2
	OpClose        = 8
	OpPing         = 9
	OpPong         = 10
)

const (
	CloseNormalClosure       = 1000
	CloseGoingAway           = 1001
	CloseProtocolError       = 1002
	CloseUnsupportedData     = 1003
	CloseNoStatusReceived    = 1005
	CloseAbnormalClosure     = 1006
	CloseInvalidFramePayload = 1007
	ClosePolicyViolation     = 1008
	CloseMessageTooBig       = 1009
	CloseMandatoryExtension  = 1010
	CloseInternalError       = 1011
	CloseServiceRestart      = 1012
	CloseTryAgainLater       = 1013
	CloseTLSHandshake        = 1015
)

type Frame struct {
	Fin     bool
	Opcode  byte
	Mask    bool
	MaskKey [4]byte
	Payload []byte
}

func NewFrame(fin bool, opcode byte, mask bool, maskKey [4]byte, payload []byte) *Frame {
	return &Frame{
		Fin:     fin,
		Opcode:  opcode,
		Mask:    mask,
		MaskKey: maskKey,
		Payload: payload,
	}
}

func NewTextFrame(payload []byte) *Frame {
	return NewFrame(true, OpText, false, [4]byte{}, payload)
}

func NewCloseFrame(code uint16, reason string) *Frame {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, code)
	buf.Write([]byte(reason))
	return NewFrame(true, OpClose, false, [4]byte{}, buf.Bytes())
}

func ReadFrame(br *bufio.Reader) (*Frame, error) {
	frame := &Frame{}
	controlByte, err := readByte(br)
	if err != nil {
		return nil, err
	}
	frame.Fin = controlByte>>7 == 1
	frame.Opcode = controlByte & 0x0f

	lengthByte, err := readByte(br)
	if err != nil {
		return nil, err
	}
	mask := lengthByte>>7 == 1
	frame.Mask = mask
	payloadLengthByte := lengthByte & 0x7f
	length := int(payloadLengthByte)
	switch payloadLengthByte {
	case 126:
		lengthValueBuf, err := readBytes(br, 2)
		if err != nil {
			return nil, err
		}
		length = int(binary.BigEndian.Uint16(lengthValueBuf))
	case 127:
		lengthValueBuf, err := readBytes(br, 8)
		if err != nil {
			return nil, err
		}
		length = int(binary.BigEndian.Uint64(lengthValueBuf))
	}

	if mask {
		maskBuf, err := readBytes(br, 4)
		if err != nil {
			return nil, err
		}
		frame.MaskKey = [4]byte(maskBuf)
	}

	payloadBuf, err := readBytes(br, length)
	if err != nil {
		return nil, err
	}
	frame.Payload = payloadBuf

	return frame, nil
}

func readBytes(br *bufio.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := br.Read(buf); err != nil && err != io.EOF {
		return nil, err
	}
	return buf, nil
}

func readByte(br *bufio.Reader) (byte, error) {
	b, err := readBytes(br, 1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (f Frame) MaskPayload() {
	if !f.Mask {
		return
	}

	if len(f.MaskKey) != 4 {
		return
	}

	maskKey := f.MaskKey
	payload := f.Payload
	for i := 0; i < len(payload); i++ {
		payload[i] ^= maskKey[i%4]
	}
}

func (f Frame) Bytes() []byte {
	buf := new(bytes.Buffer)

	fin := byte(0)
	if f.Fin {
		fin = 1
	}
	controlByte := fin << 7
	controlByte |= 0 << 6 // rsv1
	controlByte |= 0 << 5 // rsv2
	controlByte |= 0 << 4 // rsv3
	controlByte |= f.Opcode & 0x0f
	buf.WriteByte(controlByte)

	payloadLength := len(f.Payload)
	lengthByte := byte(0)
	if payloadLength < 126 {
		lengthByte = byte(payloadLength)
	} else if payloadLength < 0xffff {
		lengthByte = 126
	} else {
		lengthByte = 127
	}
	mask := byte(0)
	if f.Mask {
		mask = 1
	}
	lengthByte |= mask << 7
	buf.WriteByte(lengthByte)

	if payloadLength >= 126 && payloadLength < 0xffff {
		binary.Write(buf, binary.BigEndian, uint16(payloadLength))
	} else if payloadLength >= 0xffff {
		binary.Write(buf, binary.BigEndian, uint64(payloadLength))
	}

	if f.Mask {
		buf.Write(f.MaskKey[:])
	}

	buf.Write(f.Payload)

	return buf.Bytes()
}
