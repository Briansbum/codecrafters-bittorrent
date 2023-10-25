package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

type mode int

const (
	initial mode = iota
	str
	integer
	list
	dict
)

func newNonNilInterface() interface{} {
	return *new(interface{})
}

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(bencodedString string) (interface{}, error) {
	stack := []interface{}{}

	modes := []mode{initial}

	current := newNonNilInterface()

	for len(bencodedString) != 0 {
		fmt.Println(bencodedString)
		res := newNonNilInterface()
		if unicode.IsDigit(rune(bencodedString[0])) {
			newRes, newWork, err := decodeString(bencodedString)
			if err != nil {
				return nil, err
			}
			bencodedString = newWork
			res = newRes
		} else if bencodedString[0] == 'i' {
			newRes, newWork, err := decodeNumber(bencodedString)
			if err != nil {
				return nil, err
			}
			bencodedString = newWork
			res = newRes
		} else if bencodedString[0] == 'l' {
			newInterface := new([]interface{})
			current = *newInterface
			modes = append(modes, list)
			bencodedString = bencodedString[1:]
		} else if bencodedString[0] == 'd' {
			return nil, errors.New("dictionaries not implemented yet")
		} else if bencodedString[0] == 'e' {
			modes = modes[:len(modes)-1]
			current = newNonNilInterface()
			bencodedString = bencodedString[1:]
		} else {
			return nil, errors.New("unable to determine action to take for remaining work:" + bencodedString)
		}

		if len(modes) == 0 {
			break
		}

		switch lastMode(modes) {
		case list:
			if current != nil {
				current = append(current.([]interface{}), res)
			} else {
				current = []interface{}{res}
			}
		case dict:
			return nil, errors.New("dict not implemented")
		default:
			stack = append(stack, res)
		}
	}

	return stack, nil
}

func lastMode(modes []mode) mode {
	if len(modes) > 1 {
		return modes[len(modes)-1]
	}
	return modes[0]
}

func decodeString(b string) (string, string, error) {
	var firstColonIndex int

	for i := 0; i < len(b); i++ {
		if b[i] == ':' {
			firstColonIndex = i
			break
		}
	}

	lengthStr := b[:firstColonIndex]

	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", "", err
	}

	return b[firstColonIndex+1 : firstColonIndex+1+length], b[firstColonIndex+1+length:], nil
}

func decodeNumber(b string) (int, string, error) {
	e := strings.Index(b, "e")
	out, err := strconv.Atoi(b[1:e])
	if err != nil {
		return 0, "", err
	}
	return out, b[e:], nil
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		decoded, err := decodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, err := json.Marshal(decoded)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
