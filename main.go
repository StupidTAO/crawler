package main

import (
	"bytes"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/StupidTAO/crawler/collect"
	"github.com/StupidTAO/crawler/proxy"
	"regexp"
	"time"
)

// tag v0.0.5
var headerRe = regexp.MustCompile(`<div class="small_cardcontent__BTALp"[\s\S]*?<h2>([\s\S]*?)</h2>`)

// tag v0.0.9
func main() {
	proxyURLs := []string{"http://127.0.0.1:7890", "http://127.0.0.1:7890"}
	p, err := proxy.RoundRobinProxySwitcher(proxyURLs...)
	if err != nil {
		fmt.Println("RoundRobinProxySwitcher failed")
		return
	}

	url := "https://www.google.com"
	//url := "https://www.thepaper.cn/"
	//url := "https://book.douban.com/subject/1007305/"
	var f collect.Fetcher = collect.BrowserFetch{
		Timeout: 3000 * time.Millisecond,
		Proxy:   p,
	}
	body, err := f.Get(url)
	if err != nil {
		fmt.Println("get read content failed:%v", err)
		return
	}
	fmt.Println(string(body))

	// 加载HTML文档
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		fmt.Println("load html read content failed:%v", err)
	}

	doc.Find("div.small_cardcontent__BTALp h2").Each(func(i int, s *goquery.Selection) {
		// 获取匹配标签中的文本
		title := s.Text()
		fmt.Printf("Review %d: %s\n", i, title)
	})

}
