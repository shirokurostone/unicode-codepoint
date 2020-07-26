package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"reflect"
	"testing"
)

type ParseResult struct {
	token *Token
	err   error
}

type TestData struct {
	input    []byte
	expected []ParseResult
}

func TestUtf8ParserParse(t *testing.T) {

	utf8Cases := []TestData{
		// 1-4バイトの文字がパースできることを確認する
		TestData{
			input: []byte{
				0x61,       // a
				0xc3, 0x80, // À
				0xe3, 0x81, 0x82, // あ
				0xf0, 0xa9, 0xb8, 0xbd}, // 𩸽
			expected: []ParseResult{
				ParseResult{
					token: NewToken('a', TypeOk, []byte{0x61}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('À', TypeOk, []byte{0xc3, 0x80}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('あ', TypeOk, []byte{0xe3, 0x81, 0x82}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('𩸽', TypeOk, []byte{0xf0, 0xa9, 0xb8, 0xbd}),
					err:   nil,
				},
				ParseResult{
					token: nil,
					err:   io.EOF,
				},
			},
		},
		// 2-4バイト文字が途中で終端しているとき TypeInvalidByteSequence を返すことを確認する
		TestData{
			input: []byte{
				0xc0,
				0xe0, 0x80,
				0xf0, 0x80, 0x80,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeInvalidByteSequence, []byte{0xc0}),
					err:   nil,
				},
				ParseResult{
					token: NewToken(0, TypeInvalidByteSequence, []byte{0xe0, 0x80}),
					err:   nil,
				},
				ParseResult{
					token: NewToken(0, TypeInvalidByteSequence, []byte{0xf0, 0x80, 0x80}),
					err:   io.EOF,
				},
			},
		},

		// 冗長なエンコーディングのとき TypeRedundantEncoding を返すことを確認する
		TestData{
			input: []byte{
				0xc1, 0xa1,
				0xe0, 0x81, 0xa1,
				0xf0, 0x80, 0x81, 0xa1,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken('a', TypeRedundantEncoding, []byte{0xc1, 0xa1}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('a', TypeRedundantEncoding, []byte{0xe0, 0x81, 0xa1}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('a', TypeRedundantEncoding, []byte{0xf0, 0x80, 0x81, 0xa1}),
					err:   nil,
				},
				ParseResult{
					token: nil,
					err:   io.EOF,
				},
			},
		},
		// UTF-8に表れない不正なバイトの場合 TypeInvalidByteSequence を返すことを確認する
		TestData{
			input: []byte{
				0xff,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeInvalidByteSequence, []byte{0xff}),
					err:   nil,
				},
				ParseResult{
					token: nil,
					err:   io.EOF,
				},
			},
		},
	}

	for i, c := range utf8Cases {
		reader := bufio.NewReader(bytes.NewReader(c.input))
		parser := NewParser(reader, 8, nil)

		for j, r := range c.expected {
			actual, err := parser.parse()

			if !reflect.DeepEqual(r.token, actual) {
				t.Errorf("[%d,%d] expected: %#v, actual %#v", i, j, r.token, actual)
			}

			if !reflect.DeepEqual(r.err, err) {
				t.Errorf("[%d,%d] expected: %#v, actual %#v", i, j, r.err, err)
			}
		}

	}

}

func TestUtf16ParserBeParse(t *testing.T) {

	utf16BeCases := []TestData{
		// UTF-8のケースと同じ文字がパースできることを確認する
		TestData{
			input: []byte{
				0x00, 0x61, // a
				0x00, 0xc0, // À
				0x30, 0x42, // あ
				0xd8, 0x67, 0xde, 0x3d, // 𩸽
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken('a', TypeOk, []byte{0x00, 0x61}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('À', TypeOk, []byte{0x00, 0xc0}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('あ', TypeOk, []byte{0x30, 0x42}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('𩸽', TypeOk, []byte{0xd8, 0x67, 0xde, 0x3d}),
					err:   nil,
				},
				ParseResult{
					token: nil,
					err:   io.EOF,
				},
			},
		},
		// 端数のバイトが存在するとき TypeInvalidByteSequence を返すことを確認する
		TestData{
			input: []byte{
				0x61,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeInvalidByteSequence, []byte{0x61}),
					err:   io.ErrUnexpectedEOF,
				},
			},
		},
		// 上位サロゲートの後続に下位サロゲートが存在しないとき TypeIncompleteSurrogatePair を返すことを確認する
		TestData{
			input: []byte{
				0xd8, 0x00,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeIncompleteSurrogatePair, []byte{0xd8, 0x00}),
					err:   io.EOF,
				},
			},
		},
		// 上位サロゲートの後続に下位サロゲート以外の文字が存在するとき TypeIncompleteSurrogatePair を返すことを確認する
		TestData{
			input: []byte{
				0xd8, 0x00,
				0x00, 0x61,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeIncompleteSurrogatePair, []byte{0xd8, 0x00}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('a', TypeOk, []byte{0x00, 0x61}),
					err:   nil,
				},
				ParseResult{
					token: nil,
					err:   io.EOF,
				},
			},
		},
		// 上位サロゲートの後続以外に下位サロゲートが存在したとき TypeIncompleteSurrogatePair を返すことを確認する
		TestData{
			input: []byte{
				0xdc, 0x00,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeIncompleteSurrogatePair, []byte{0xdc, 0x00}),
					err:   nil,
				},
				ParseResult{
					token: nil,
					err:   io.EOF,
				},
			},
		},
	}

	for i, c := range utf16BeCases {
		reader := bufio.NewReader(bytes.NewReader(c.input))
		parser := NewParser(reader, 16, binary.BigEndian)

		for j, r := range c.expected {
			actual, err := parser.parse()

			if !reflect.DeepEqual(r.token, actual) {
				t.Errorf("[%d,%d] expected: %#v, actual %#v", i, j, r.token, actual)
			}

			if !reflect.DeepEqual(r.err, err) {
				t.Errorf("[%d,%d] expected: %#v, actual %#v", i, j, r.err, err)
			}
		}

	}

}

func TestUtf16ParserLeParse(t *testing.T) {

	utf16LeCases := []TestData{
		// UTF-8のケースと同じ文字がパースできることを確認する
		TestData{
			input: []byte{
				0x61, 0x00, // a
				0xc0, 0x00, // À
				0x42, 0x30, // あ
				0x67, 0xd8, 0x3d, 0xde, // 𩸽
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken('a', TypeOk, []byte{0x61, 0x00}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('À', TypeOk, []byte{0xc0, 0x00}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('あ', TypeOk, []byte{0x42, 0x30}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('𩸽', TypeOk, []byte{0x67, 0xd8, 0x3d, 0xde}),
					err:   nil,
				},
				ParseResult{
					token: nil,
					err:   io.EOF,
				},
			},
		},
		// 端数のバイトが存在するとき TypeInvalidByteSequence を返すことを確認する
		TestData{
			input: []byte{
				0x61,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeInvalidByteSequence, []byte{0x61}),
					err:   io.ErrUnexpectedEOF,
				},
			},
		},
		// 上位サロゲートの後続に下位サロゲートが存在しないとき TypeIncompleteSurrogatePair を返すことを確認する
		TestData{
			input: []byte{
				0x00, 0xd8,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeIncompleteSurrogatePair, []byte{0x00, 0xd8}),
					err:   io.EOF,
				},
			},
		},
		// 上位サロゲートの後続に下位サロゲート以外の文字が存在するとき TypeIncompleteSurrogatePair を返すことを確認する
		TestData{
			input: []byte{
				0x00, 0xd8,
				0x61, 0x00,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeIncompleteSurrogatePair, []byte{0x00, 0xd8}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('a', TypeOk, []byte{0x61, 0x00}),
					err:   nil,
				},
				ParseResult{
					token: nil,
					err:   io.EOF,
				},
			},
		},
		// 上位サロゲートの後続以外に下位サロゲートが存在したとき TypeIncompleteSurrogatePair を返すことを確認する
		TestData{
			input: []byte{
				0x00, 0xdc,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeIncompleteSurrogatePair, []byte{0x00, 0xdc}),
					err:   nil,
				},
				ParseResult{
					token: nil,
					err:   io.EOF,
				},
			},
		},
	}

	for i, c := range utf16LeCases {
		reader := bufio.NewReader(bytes.NewReader(c.input))
		parser := NewParser(reader, 16, binary.LittleEndian)

		for j, r := range c.expected {
			actual, err := parser.parse()

			if !reflect.DeepEqual(r.token, actual) {
				t.Errorf("[%d,%d] expected: %#v, actual %#v", i, j, r.token, actual)
			}

			if !reflect.DeepEqual(r.err, err) {
				t.Errorf("[%d,%d] expected: %#v, actual %#v", i, j, r.err, err)
			}
		}

	}

}

func TestUtf32ParserBeParse(t *testing.T) {

	utf32BeCases := []TestData{
		// UTF-8のケースと同じ文字がパースできることを確認する
		TestData{
			input: []byte{
				0x00, 0x00, 0x00, 0x61, // a
				0x00, 0x00, 0x00, 0xc0, // À
				0x00, 0x00, 0x30, 0x42, // あ
				0x00, 0x02, 0x9e, 0x3d, // 𩸽
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken('a', TypeOk, []byte{0x00, 0x00, 0x00, 0x61}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('À', TypeOk, []byte{0x00, 0x00, 0x00, 0xc0}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('あ', TypeOk, []byte{0x00, 0x00, 0x30, 0x42}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('𩸽', TypeOk, []byte{0x00, 0x02, 0x9e, 0x3d}),
					err:   nil,
				},
				ParseResult{
					token: nil,
					err:   io.EOF,
				},
			},
		},
		// 端数のバイトが存在するとき TypeInvalidByteSequence を返すことを確認する
		TestData{
			input: []byte{
				0x00, 0x61,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeInvalidByteSequence, []byte{0x00, 0x61}),
					err:   io.ErrUnexpectedEOF,
				},
			},
		},
		// 面11以降が不正と判定されることを確認する
		TestData{
			input: []byte{
				0x00, 0x11, 0x00, 0x00,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeInvalidByteSequence, []byte{0x00, 0x11, 0x00, 0x00}),
					err:   nil,
				},
			},
		},
	}

	for i, c := range utf32BeCases {
		reader := bufio.NewReader(bytes.NewReader(c.input))
		parser := NewParser(reader, 32, binary.BigEndian)

		for j, r := range c.expected {
			actual, err := parser.parse()

			if !reflect.DeepEqual(r.token, actual) {
				t.Errorf("[%d,%d] expected: %#v, actual %#v", i, j, r.token, actual)
			}

			if !reflect.DeepEqual(r.err, err) {
				t.Errorf("[%d,%d] expected: %#v, actual %#v", i, j, r.err, err)
			}
		}

	}

}

func TestUtf32ParserLeParse(t *testing.T) {

	utf32LeCases := []TestData{
		// UTF-8のケースと同じ文字がパースできることを確認する
		TestData{
			input: []byte{
				0x61, 0x00, 0x00, 0x00, // a
				0xc0, 0x00, 0x00, 0x00, // À
				0x42, 0x30, 0x00, 0x00, // あ
				0x3d, 0x9e, 0x02, 0x00, // 𩸽
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken('a', TypeOk, []byte{0x61, 0x00, 0x00, 0x00}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('À', TypeOk, []byte{0xc0, 0x00, 0x00, 0x00}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('あ', TypeOk, []byte{0x42, 0x30, 0x00, 0x00}),
					err:   nil,
				},
				ParseResult{
					token: NewToken('𩸽', TypeOk, []byte{0x3d, 0x9e, 0x02, 0x00}),
					err:   nil,
				},
				ParseResult{
					token: nil,
					err:   io.EOF,
				},
			},
		},
		// 端数のバイトが存在するとき TypeInvalidByteSequence を返すことを確認する
		TestData{
			input: []byte{
				0x00, 0x61,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeInvalidByteSequence, []byte{0x00, 0x61}),
					err:   io.ErrUnexpectedEOF,
				},
			},
		},
		// 面11以降が不正と判定されることを確認する
		TestData{
			input: []byte{
				0x00, 0x00, 0x11, 0x00,
			},
			expected: []ParseResult{
				ParseResult{
					token: NewToken(0, TypeInvalidByteSequence, []byte{0x00, 0x00, 0x11, 0x00}),
					err:   nil,
				},
			},
		},
	}

	for i, c := range utf32LeCases {
		reader := bufio.NewReader(bytes.NewReader(c.input))
		parser := NewParser(reader, 32, binary.LittleEndian)

		for j, r := range c.expected {
			actual, err := parser.parse()

			if !reflect.DeepEqual(r.token, actual) {
				t.Errorf("[%d,%d] expected: %#v, actual %#v", i, j, r.token, actual)
			}

			if !reflect.DeepEqual(r.err, err) {
				t.Errorf("[%d,%d] expected: %#v, actual %#v", i, j, r.err, err)
			}
		}

	}

}
