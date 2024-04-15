package collect

import (
	"bufio"
	"fmt"
	"github.com/StupidTAO/crawler/proxy"
	"go.uber.org/zap"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"io/ioutil"
	"net/http"
	"time"
)

type Fetcher interface {
	Get(req *Request) ([]byte, error)
}

type BaseFetch struct {
}

func (BaseFetch) Get(request *Request) ([]byte, error) {
	resp, err := http.Get(request.Url)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error status code:%d\n", resp.StatusCode)
		return nil, err
	}
	bodyReader := bufio.NewReader(resp.Body)
	e := DeterminEncoding(bodyReader)
	utf8Reader := transform.NewReader(bodyReader, e.NewDecoder())
	return ioutil.ReadAll(utf8Reader)
}

func DeterminEncoding(r *bufio.Reader) encoding.Encoding {

	bytes, err := r.Peek(1024)

	if err != nil {
		fmt.Printf("fetch error:%v\n", err)
		return unicode.UTF8
	}

	e, _, _ := charset.DetermineEncoding(bytes, "")
	return e
}

type BrowserFetch struct {
	Timeout time.Duration
	Proxy   proxy.ProxyFunc
	Logger  *zap.Logger
}

// 模拟浏览器访问
func (b BrowserFetch) Get(request *Request) ([]byte, error) {
	client := &http.Client{
		Timeout: b.Timeout,
	}

	if b.Proxy != nil {
		transport := http.DefaultTransport.(*http.Transport)
		transport.Proxy = b.Proxy
		client.Transport = transport
	}
	req, err := http.NewRequest("GET", request.Url, nil)
	if err != nil {
		return nil, fmt.Errorf("get url failed:%v", err)
	}

	//设置cookie
	if len(request.Task.Cookie) > 0 {
		req.Header.Set("Cookie", request.Task.Cookie)
	}

	//模拟浏览器
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.149 Safari/537.36")

	resp, err := client.Do(req)

	time.Sleep(request.Task.WaitTime)

	if err != nil {
		b.Logger.Error("get error: ", zap.Error(err))
		return nil, err
	}

	bodyReader := bufio.NewReader(resp.Body)
	e := DeterminEncoding(bodyReader)
	utf8Reader := transform.NewReader(bodyReader, e.NewDecoder())

	return ioutil.ReadAll(utf8Reader)
}
