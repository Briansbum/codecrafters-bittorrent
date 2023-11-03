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

func decodeBencode(b string) (interface{}, string, error) {
	if strings.HasPrefix(b, "i") {
		res, left, err := decodeNumber(b)
		if err != nil {
			return nil, "", err
		}
		return res, left, nil
	}

	if strings.HasPrefix(b, "l") {
		list := []interface{}{}
		var res interface{}
		var err error

		left := b[1:]

		for !strings.HasPrefix(left, "e") {
			res, left, err = decodeBencode(left)
			if err != nil {
				return nil, "", err
			}
			list = append(list, res)
		}
		return list, left[1:], nil
	}

	if unicode.IsDigit(rune(b[0])) {
		res, left, err := decodeString(b)
		if err != nil {
			return nil, "", err
		}
		return res, left, nil
	}

	if strings.HasPrefix(b, "d") {
		dict := map[string]interface{}{}
		left := b[1:]
		var key string
		var res interface{}
		var err error

		for !strings.HasPrefix(left, "e") {
			key, left, err = decodeString(left)
			if err != nil {
				return nil, "", err
			}
			res, left, err = decodeBencode(left)
			if err != nil {
				return nil, "", err
			}
			dict[key] = res
		}
		return dict, left, nil
	}

	return nil, "", errors.New("expected bencoded string, got " + b)
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
	return out, b[e+1:], nil
}

func main() {
	command := os.Args[1]

	switch command {
	case "decode":
		bencodedValue := os.Args[2]

		decoded, _, err := decodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, err := json.Marshal(decoded)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(jsonOutput))
	case "info":
		filename := os.Args[2]

		torrentFileBytes, err := os.ReadFile(filename)
		if err != nil {
			panic(err)
		}

		decoded, _, err := decodeBencode(string(torrentFileBytes))
		if err != nil {
			panic(err)
		}
		decodedMap := decoded.(map[string]interface{})
		fmt.Println("Tracker URL: " + decodedMap["announce"].(string))

		fmt.Printf("Length: %d", decodedMap["info"].(map[string]interface{})["length"])
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
