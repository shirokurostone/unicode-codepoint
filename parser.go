package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/runenames"
)

const (
	TypeOk = iota
	TypeInvalidByteSequence
	TypeRedundantEncoding
	TypeIncompleteSurrogatePair
)

func NewParser(reader *bufio.Reader, bit int, byteOrder binary.ByteOrder) Parser {

	if bit == 8 {
		return &utf8Parser{
			baseParser: baseParser{
				reader: reader,
			},
		}
	} else if bit == 16 {
		return &utf16Parser{
			baseParser: baseParser{
				reader: reader,
			},
			ByteOrder: byteOrder,
		}
	} else if bit == 32 {
		return &utf32Parser{
			baseParser: baseParser{
				reader: reader,
			},
			ByteOrder: byteOrder,
		}
	}

	return nil
}

type Parser interface {
	parse() (*Token, error)
}

type Token struct {
	Rune  rune
	Bytes []byte
	Type  int
}

func NewToken(Rune rune, Type int, Bytes []byte) *Token {
	token := Token{
		Rune:  Rune,
		Type:  Type,
		Bytes: Bytes,
	}
	return &token
}

func (t *Token) String() string {
	s := []string{}
	for _, b := range t.Bytes {
		s = append(s, fmt.Sprintf("%02x", b))
	}

	var c, name string
	if !unicode.IsControl(t.Rune) {
		c = fmt.Sprintf("%c", t.Rune)
		name = runenames.Name(t.Rune)
	} else {
		if val, ok := controlCodeSymbols[t.Rune]; ok {
			c = val
		} else {
			c = "(control)"
		}
		name = runenames.Name(t.Rune)
		if val, ok := controlCodeAliases[t.Rune]; ok {
			name += " " + val
		}
	}

	if t.Type == TypeOk {
		return fmt.Sprintf("%s\t%U\t%s\t%s", c, t.Rune, strings.Join(s, " "), name)
	} else if t.Type == TypeRedundantEncoding {
		return fmt.Sprintf("%s\t%U\t%s\t[Redundant encoding]%s", c, t.Rune, strings.Join(s, " "), name)
	}
	return fmt.Sprintf("\t\t%s\t", strings.Join(s, " "))
}

type baseParser struct {
	reader *bufio.Reader
}

func (p *baseParser) readByte() (uint8, error) {
	return p.reader.ReadByte()
}

func (p *baseParser) readFull(buf []byte) (int, error) {
	return io.ReadFull(p.reader, buf)
}

func (p *baseParser) peekByte() (uint8, error) {
	bs, err := p.peek(1)
	if err != nil || len(bs) != 1 {
		return 0, err
	}
	return bs[0], nil
}

func (p *baseParser) peek(n int) ([]byte, error) {
	return p.reader.Peek(n)
}

type utf8Parser struct {
	baseParser
}

func (p *utf8Parser) parse() (*Token, error) {
	var b1, b2, b3, b4 uint8
	var err error

	b1, err = p.readByte()
	if err != nil {
		return nil, err
	}

	if b1 <= 0x7f {
		return NewToken(rune(b1), TypeOk, []byte{b1}), nil
	} else if b1 <= 0xbf {
		return NewToken(0, TypeInvalidByteSequence, []byte{b1}), err
	} else if b1 <= 0xdf {

		if b2, err = p.peekByte(); err != nil {
			return NewToken(0, TypeInvalidByteSequence, []byte{b1}), err
		}
		if b2&0xc0 != 0x80 {
			return NewToken(0, TypeInvalidByteSequence, []byte{b1}), err
		}
		p.readByte()

		r := rune(b1&0x1f)<<6 | rune(b2)&0x3f
		token := NewToken(r, TypeOk, []byte{b1, b2})
		if r < 0x80 {
			token.Type = TypeRedundantEncoding
		}
		return token, nil
	} else if b1 <= 0xef {

		if b2, err = p.peekByte(); err != nil {
			return NewToken(0, TypeInvalidByteSequence, []byte{b1}), err
		}
		if b2&0xc0 != 0x80 {
			return NewToken(0, TypeInvalidByteSequence, []byte{b1}), err
		}
		p.readByte()

		if b3, err = p.peekByte(); err != nil {
			return NewToken(0, TypeInvalidByteSequence, []byte{b1, b2}), err
		}
		if b3&0xc0 != 0x80 {
			return NewToken(0, TypeInvalidByteSequence, []byte{b1, b2}), err
		}
		p.readByte()

		r := rune(b1&0x0f)<<12 | (rune(b2)&0x3f)<<6 | (rune(b3) & 0x3f)
		token := NewToken(r, TypeOk, []byte{b1, b2, b3})
		if r < 0x800 {
			token.Type = TypeRedundantEncoding
		}
		return token, nil
	} else if b1 <= 0xf7 {

		if b2, err = p.peekByte(); err != nil {
			return NewToken(0, TypeInvalidByteSequence, []byte{b1}), err
		}
		if b2&0xc0 != 0x80 {
			return NewToken(0, TypeInvalidByteSequence, []byte{b1}), err
		}
		p.readByte()

		if b3, err = p.peekByte(); err != nil {
			return NewToken(0, TypeInvalidByteSequence, []byte{b1, b2}), err
		}
		if b3&0xc0 != 0x80 {
			return NewToken(0, TypeInvalidByteSequence, []byte{b1, b2}), err
		}
		p.readByte()

		if b4, err = p.peekByte(); err != nil {
			return NewToken(0, TypeInvalidByteSequence, []byte{b1, b2, b3}), err
		}
		if b4&0xc0 != 0x80 {
			return NewToken(0, TypeInvalidByteSequence, []byte{b1, b2, b3}), err
		}
		p.readByte()

		r := rune(b1&0x07)<<18 | (rune(b2)&0x3f)<<12 | (rune(b3)&0x3f)<<6 | (rune(b4) & 0x3f)
		token := NewToken(r, TypeOk, []byte{b1, b2, b3, b4})
		if r < 0x10000 {
			token.Type = TypeRedundantEncoding
		}
		return token, nil
	} else {
		return NewToken(0, TypeInvalidByteSequence, []byte{b1}), err
	}

}

type utf16Parser struct {
	baseParser
	ByteOrder binary.ByteOrder
}

func (p *utf16Parser) parse() (*Token, error) {

	bytes := make([]byte, 2)
	if n, err := p.readFull(bytes); err != nil {
		if n == 0 {
			return nil, err
		}
		return NewToken(0, TypeInvalidByteSequence, bytes[:n]), err
	}
	r1 := rune(p.ByteOrder.Uint16(bytes))

	if p.isHighSurrogate(r1) {
		bytes2, err := p.peek(2)
		if err != nil || len(bytes2) != 2 {
			return NewToken(0, TypeIncompleteSurrogatePair, bytes), err
		}

		r2 := rune(p.ByteOrder.Uint16(bytes2))
		if !p.isLowSurrogate(r2) {
			return NewToken(0, TypeIncompleteSurrogatePair, bytes), err
		}

		c := (r1&0x3ff)<<10 | r2&0x3ff + 0x10000
		p.readByte()
		p.readByte()

		return NewToken(c, TypeOk, append(bytes, bytes2...)), nil
	} else if p.isLowSurrogate(r1) {
		return NewToken(0, TypeIncompleteSurrogatePair, bytes), nil
	}

	return NewToken(r1, TypeOk, bytes), nil

}

func (p *utf16Parser) isHighSurrogate(r rune) bool {
	return 0xd800 <= r && r <= 0xdbff
}

func (p *utf16Parser) isLowSurrogate(r rune) bool {
	return 0xdc00 <= r && r <= 0xdfff
}

type utf32Parser struct {
	baseParser
	ByteOrder binary.ByteOrder
}

func (p *utf32Parser) parse() (*Token, error) {

	bytes := make([]byte, 4)
	if n, err := p.readFull(bytes); err != nil {
		if n == 0 {
			return nil, err
		}
		return NewToken(0, TypeInvalidByteSequence, bytes[:n]), err
	}
	r := rune(p.ByteOrder.Uint32(bytes))

	if r > 0x10ffff {
		return NewToken(0, TypeInvalidByteSequence, bytes), nil
	}

	return NewToken(r, TypeOk, bytes), nil
}
