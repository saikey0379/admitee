package utils

import (
	"fmt"
	"net"
	"net/http"
	"time"
	"bytes"
	"strings"
	"io/ioutil"
)

func RestApiGet(url string) (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				conn, err := net.DialTimeout(netw, addr, time.Second*10)
				if err != nil {
					return nil, err
				}
				conn.SetDeadline(time.Now().Add(time.Second * 10))
				return conn, nil
			},
			ResponseHeaderTimeout: time.Second * 10,
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	if resp != nil {
		defer resp.Body.Close()
	}

	ret, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(ret)), nil
}

func RestApiPost(url string, body string) (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				conn, err := net.DialTimeout(netw, addr, time.Second*3)
				if err != nil {
					return nil, err
				}
				conn.SetDeadline(time.Now().Add(time.Second * 3))
				return conn, nil
			},
			ResponseHeaderTimeout: time.Second * 3,
		},
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(body)))
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	if resp != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("FAILURE: Http status code[%v]", resp.StatusCode)
	}
	ret, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(ret)), nil
}
