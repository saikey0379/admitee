package utils

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

const TimeOut = 10

func RestApiGet(url string) (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				conn, err := net.DialTimeout(netw, addr, time.Second*TimeOut)
				if err != nil {
					return nil, err
				}
				conn.SetDeadline(time.Now().Add(time.Second * TimeOut))
				return conn, nil
			},
			ResponseHeaderTimeout: time.Second * TimeOut,
		},
	}

	resp, err := client.Get(url)
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

func RestApiPost(url string, body string) (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				conn, err := net.DialTimeout(netw, addr, time.Second*TimeOut)
				if err != nil {
					return nil, err
				}
				conn.SetDeadline(time.Now().Add(time.Second * TimeOut))
				return conn, nil
			},
			ResponseHeaderTimeout: time.Second * TimeOut,
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
