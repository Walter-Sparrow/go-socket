package v13

import (
	"bytes"
	"encoding/binary"
)

const (
	OpContinuation = 0
	OpText         = 1
	OpBinary       = 2
	OpClose        = 8
	OpPing         = 9
	OpPong         = 10
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
