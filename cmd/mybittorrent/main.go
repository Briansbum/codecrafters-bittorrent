package main

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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

func torrentInfo(t interface{}) (string, error) {
	decodedMap := t.(map[string]interface{})

	info := fmt.Sprintf("Tracker URL: %s\n", decodedMap["announce"].(string))

	info = info + fmt.Sprintf("Length: %d\n", decodedMap["info"].(map[string]interface{})["length"])

	hash, err := infoHash(decodedMap)
	if err != nil {
		return "", err
	}

	info = info + fmt.Sprintf("Info Hash: %x\n", hash)
	info = info + fmt.Sprintf("Piece Length: %d\n", decodedMap["info"].(map[string]interface{})["piece length"].(int))
	info = info + fmt.Sprintf("Piece Hashes: \n")

	pieces := []byte(decodedMap["info"].(map[string]interface{})["pieces"].(string))
	for i := len(pieces) / 20; i != 0; i-- {
		info = info + fmt.Sprintf("%x\n", pieces[:20])
		pieces = pieces[20:]
	}

	return info, nil
}

func infoHash(decodedMap map[string]interface{}) ([]byte, error) {
	infoBencoded, err := bencode(decodedMap["info"])
	if err != nil {
		return nil, err
	}
	if _, _, err := decodeBencode(infoBencoded); err != nil {
		return nil, fmt.Errorf("error decoding after encode, something went wrong with encoding: %w", err)
	}
	hasher := sha1.New()
	if _, err := hasher.Write([]byte(infoBencoded)); err != nil {
		return nil, err
	}

	return hasher.Sum(nil), nil
}

func requestPeers(decoded interface{}) ([]byte, error) {
	host := decoded.(map[string]interface{})["announce"].(string)

	hash, err := infoHash(decoded.(map[string]interface{}))
	if err != nil {
		return nil, err
	}

	params := url.Values{}

	params.Add("info_hash", string(hash))
	params.Add("peer_id", "00112233445566778899")
	params.Add("port", "6881")
	params.Add("uploaded", "0")
	params.Add("downloaded", "0")
	params.Add("left", strconv.Itoa(decoded.(map[string]interface{})["info"].(map[string]interface{})["length"].(int)))
	params.Add("compact", "1")

	resp, err := http.Get(host + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		return nil, errors.New("wrong status " + resp.Status + " when calling " + host + "?" + params.Encode())
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	decodedResp, _, err := decodeBencode(string(body))
	if err != nil {
		return nil, err
	}

	return []byte(decodedResp.(map[string]interface{})["peers"].(string)), nil
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

		info, err := torrentInfo(decoded)
		if err != nil {
			panic(err)
		}

		fmt.Println(info)
	case "peers":
		filename := os.Args[2]

		torrentFileBytes, err := os.ReadFile(filename)
		if err != nil {
			panic(err)
		}

		decoded, _, err := decodeBencode(string(torrentFileBytes))
		if err != nil {
			panic(err)
		}

		peers, err := requestPeers(decoded)
		if err != nil {
			panic(err)
		}

		for i := 0; i < len(peers)/6; i++ {
			offset := i * 6
			peer := peers[offset : offset+6]
			ip := net.IP(peer[:4])
			port := binary.BigEndian.Uint16([]byte{peers[offset+4], peers[offset+5]})
			fmt.Printf("%s:%d\n", ip.String(), port)
		}
	case "handshake":
		filename := os.Args[2]
		peer := strings.Split(os.Args[3], ":")

		peerIP := net.IP([]byte(peer[0]))
		peerPort, err := strconv.Atoi(peer[1])
		if err != nil {
			panic(err)
		}

		conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{
			IP:   peerIP,
			Port: peerPort,
		})
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		conn.Write([]byte{})
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
