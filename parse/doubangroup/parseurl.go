// 该模块只是在做链接、以及链接内容的解析；不涉及并发逻辑q
package doubangroup

import (
	"fmt"
	"github.com/StupidTAO/crawler/collect"
	"regexp"
)

const urlListRe = `(https://www.douban.com/group/topic/[0-9a-z]+/)"[^>]*>([^<]+)</a>`

func ParseURL(contents []byte, req *collect.Request) collect.ParseResult {
	re := regexp.MustCompile(urlListRe)

	matches := re.FindAllSubmatch(contents, -1)
	result := collect.ParseResult{}

	fmt.Println("ParseURL() len(matches) = ", len(matches))
	for _, m := range matches {
		u := string(m[1])
		result.Requests = append(
			result.Requests, &collect.Request{
				Method: "GET",
				Task:   req.Task,
				Url:    u,
				Depth:  req.Depth + 1,
				ParseFunc: func(c []byte, request *collect.Request) collect.ParseResult {
					return GetContent(c, u)
				},
			})
	}
	return result
}

const ContentRe = `<div class="topic-content">[\s\S]*?阳台[\s\S]*?<div`

func GetContent(contents []byte, url string) collect.ParseResult {
	re := regexp.MustCompile(ContentRe)

	ok := re.Match(contents)
	if !ok {
		return collect.ParseResult{
			Items: []interface{}{},
		}
	}

	fmt.Println("GetContent() url = ", url)
	result := collect.ParseResult{
		Items: []interface{}{url},
	}

	return result
}
