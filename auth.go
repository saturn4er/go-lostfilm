package lostfilm

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"fmt"

	"golang.org/x/net/html/charset"
)

type Lostfilm struct {
	jar             *cookiejar.Jar
	email, password string
}

func NewClient(email, password string) (*Lostfilm, error) {
	result := new(Lostfilm)
	result.email, result.password = email, password
	result.jar, _ = cookiejar.New(nil)
	result.loginToLostfilm()
	return result, nil
}

func (l *Lostfilm) loginToLostfilm() error {
	params := url.Values{
		"act":  {"users"},
		"type": {"login"},
		"mail": {l.email},
		"pass": {l.password},
	}
	resp, err := l.request("POST", "https://www.lostfilm.tv/ajaxik.php?"+params.Encode())
	if err != nil {
		return err
	}
	p, err := l.decodeResponse(resp)
	resp.Body.Close()
	fmt.Println(p)
	//TODO: check for error
	return nil
}
func (l *Lostfilm) decodeResponse(res *http.Response) (string, error) {
	utf8, err := charset.NewReader(res.Body, res.Header.Get("Content-Type"))
	if err != nil {
		log.Printf("Error decoding response: %v", err)
		return "", err
	}
	b, err := ioutil.ReadAll(utf8)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
func (l *Lostfilm) request(method, endpoint string) (*http.Response, error) {
	c := http.Client{Jar: l.jar}
	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	return resp, err
}
