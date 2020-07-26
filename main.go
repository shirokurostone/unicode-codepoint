package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	var charset string

	flag.StringVar(&charset, "c", "UTF-8", "select character set (UTF-8 | UTF-16 | UTF-16BE | UTF-16LE | UTF-32 | UTF-32BE | UTF-32LE)")
	flag.Parse()

	reader := bufio.NewReader(os.Stdin)
	var parser Parser
	charset = strings.ToUpper(charset)

	if charset == "UTF-8" {
		parser = NewParser(reader, 8, nil)
	} else if charset == "UTF-16" || charset == "UTF-16BE" {
		parser = NewParser(reader, 16, binary.BigEndian)
	} else if charset == "UTF-16LE" {
		parser = NewParser(reader, 16, binary.LittleEndian)
	} else if charset == "UTF-32" || charset == "UTF-32BE" {
		parser = NewParser(reader, 32, binary.BigEndian)
	} else if charset == "UTF-32LE" {
		parser = NewParser(reader, 32, binary.LittleEndian)
	} else {
		flag.PrintDefaults()
		os.Exit(1)
	}

	for {
		token, err := parser.parse()
		if token != nil {
			fmt.Println(token)
		}
		if err != nil {
			break
		}
	}

}
