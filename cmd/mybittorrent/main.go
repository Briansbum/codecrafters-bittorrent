package main

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
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

func bencode(d interface{}) (string, error) {
	out := ""

	switch d.(type) {
	case string:
		s := d.(string)
		out = out + fmt.Sprintf("%d:%s", len(s), s)
	case int:
		out = out + fmt.Sprintf("i%de", d.(int))
	case map[string]interface{}:
		out = out + "d"

		keys := []string{}
		for k, _ := range d.(map[string]interface{}) {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] < keys[j]
		})

		for _, k := range keys {
			keyEncode, err := bencode(k)
			if err != nil {
				return "", err
			}
			out = out + keyEncode

			valueEncode, err := bencode(d.(map[string]interface{})[k])
			if err != nil {
				return "", err
			}
			out = out + valueEncode
		}
		out = out + "e"
	case []interface{}:
		out = out + "l"

		for _, li := range d.([]interface{}) {
			valueBencode, err := bencode(li)
			if err != nil {
				return "", err
			}
			out = out + valueBencode
		}

		out = out + "e"
	default:
		return "", fmt.Errorf("type of %+v must be one of string,int,list,dict but got: %T", d, d)
	}

	return out, nil
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

		info := fmt.Sprintf("Tracker URL: %s\n", decodedMap["announce"].(string))

		info = info + fmt.Sprintf("Length: %d\n", decodedMap["info"].(map[string]interface{})["length"])

		infoBencoded, err := bencode(decodedMap["info"])
		if err != nil {
			panic(err)
		}
		if _, _, err := decodeBencode(infoBencoded); err != nil {
			fmt.Printf("error decoding after encode, something went wrong with encoding: %+v\n", err)
		}

		info = info + fmt.Sprintf("Info Hash: %x\n", sha1.Sum([]byte(infoBencoded)))

		fmt.Println(info)
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
