package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"io/ioutil"
	"net/http"
	"regexp"
)

// tag v0.0.5
var headerRe = regexp.MustCompile(`<div class="small_cardcontent__BTALp"[\s\S]*?<h2>([\s\S]*?)</h2>`)

// tag v0.0.6
func main() {
	url := "https://www.thepaper.cn/"
	body, err := Fetch(url)

	if err != nil {
		fmt.Println("read content failed:%v", err)
		return
	}
	doc, err := htmlquery.Parse(bytes.NewReader(body))
	if err != nil {
		fmt.Println("htmlquery.Parse failed:%v", err)
	}
	//Xpath
	nodes := htmlquery.Find(doc, `//div[@class="small_cardcontent__BTALp"]//h2`)

	for _, node := range nodes {
		fmt.Println("fetch card ", node.FirstChild.Data)
	}
}

// tag v0.0.4
func Fetch(url string) ([]byte, error) {

	resp, err := http.Get(url)

	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error status code:%d", resp.StatusCode)
	}
	bodyReader := bufio.NewReader(resp.Body)
	e := DeterminEncoding(bodyReader)
	utf8Reader := transform.NewReader(bodyReader, e.NewDecoder())
	return ioutil.ReadAll(utf8Reader)
}

func DeterminEncoding(r *bufio.Reader) encoding.Encoding {

	bytes, err := r.Peek(1024)

	if err != nil {
		fmt.Println("fetch error:%v", err)
		return unicode.UTF8
	}

	e, _, _ := charset.DetermineEncoding(bytes, "")
	return e
}
