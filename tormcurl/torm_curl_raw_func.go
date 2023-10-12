package tormcurl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/suifengpiao14/logchan/v2"
	"github.com/suifengpiao14/torm/pkg"
	"github.com/suifengpiao14/torm/tormfunc"
)

var CURL_TIMEOUT = 30 * time.Millisecond

type RequestDTO struct {
	URL     string         `json:"url"`
	Method  string         `json:"method"`
	Header  http.Header    `json:"header"`
	Cookies []*http.Cookie `json:"cookies"`
	Body    string         `json:"body"`
}
type ResponseData struct {
	HttpStatus  string         `json:"httpStatus"`
	Header      http.Header    `json:"header"`
	Cookies     []*http.Cookie `json:"cookies"`
	Body        string         `json:"body"`
	RequestData *RequestDTO    `json:"requestData"`
}

type CURLConfig struct {
	Proxy               string `json:"proxy"`
	LogLevel            string `json:"logLevel"`
	Timeout             int    `json:"timeout"`
	KeepAlive           int    `json:"keepAlive"`
	MaxIdleConns        int    `json:"maxIdleConns"`
	MaxIdleConnsPerHost int    `json:"maxIdleConnsPerHost"`
	IdleConnTimeout     int    `json:"idleConnTimeout"`
}

func RegisterCURL(cfg CURLConfig) {

}

func InitHTTPClient(cfg *CURLConfig) *http.Client {

	maxIdleConns := 200
	maxIdleConnsPerHost := 20
	idleConnTimeout := 90
	if cfg.MaxIdleConns > 0 {
		maxIdleConns = cfg.MaxIdleConns
	}
	if cfg.MaxIdleConnsPerHost > 0 {
		maxIdleConnsPerHost = cfg.MaxIdleConnsPerHost
	}
	if cfg.IdleConnTimeout > 0 {
		idleConnTimeout = cfg.IdleConnTimeout
	}
	timeout := 10
	if cfg.Timeout > 0 {
		timeout = 10
	}
	keepAlive := 300
	if cfg.KeepAlive > 0 {
		keepAlive = cfg.KeepAlive
	}
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(timeout) * time.Second,   // 连接超时时间
			KeepAlive: time.Duration(keepAlive) * time.Second, // 连接保持超时时间
		}).DialContext,
		MaxIdleConns:        maxIdleConns,                                 // 最大连接数,默认0无穷大
		MaxIdleConnsPerHost: maxIdleConnsPerHost,                          // 对每个host的最大连接数量(MaxIdleConnsPerHost<=MaxIdleConns)
		IdleConnTimeout:     time.Duration(idleConnTimeout) * time.Second, // 多长时间未使用自动关闭连
	}
	if cfg.Proxy != "" {
		proxy, err := url.Parse(cfg.Proxy)
		if err != nil {
			panic(err)
		}
		transport.Proxy = http.ProxyURL(proxy)
	}
	httpClient := &http.Client{
		Transport: transport,
	}
	return httpClient
}

type LogName string

func (l LogName) String() string {
	return string(l)
}

const (
	LOG_INFO_CURL_RAW LogName = "LogInfoCURLRaw"
)

type LogInfoCURLRaw struct {
	HttpRaw string `json:"httpRaw"`
	Out     string `json:"out"`
	Err     error  `json:"error"`
	Level   string `json:"level"`
	logchan.EmptyLogInfo
}

func (l *LogInfoCURLRaw) GetName() logchan.LogName {
	return LOG_INFO_CURL_RAW
}
func (l *LogInfoCURLRaw) Error() error {
	return l.Err
}
func (l *LogInfoCURLRaw) GetLevel() string {
	return l.Level
}

func CURLRaw(cfg *CURLConfig, httpRaw string) (out string, err error) {
	logInfo := &LogInfoCURLRaw{
		HttpRaw: httpRaw,
		Level:   cfg.LogLevel,
	}
	defer func() {
		logInfo.Out = out
		logInfo.Err = err
		logchan.SendLogInfo(logInfo)
	}()
	reqReader, err := ReadRequest(httpRaw)
	if err != nil {
		return "", err
	}
	reqData, err := Request2RequestData(reqReader)
	if err != nil {
		return "", err
	}
	timeout := 30
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}
	timeoutStr := reqReader.Header.Get("x-http-timeout")
	if timeoutStr != "" {
		timeoutInt, _ := strconv.Atoi(timeoutStr)
		if timeoutInt > 0 {
			timeout = timeoutInt // 优先使用定制化的超时时间
		}
	}
	timeoutDuration := time.Duration(timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, reqData.Method, reqData.URL, bytes.NewReader([]byte(reqData.Body)))
	if err != nil {
		return "", err
	}

	for k, vArr := range reqData.Header {
		for _, v := range vArr {
			req.Header.Add(k, v)
		}
	}
	client := InitHTTPClient(cfg)

	rsp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer rsp.Body.Close()
	b, err := io.ReadAll(rsp.Body)
	if err != nil {
		return "", err
	}
	if err != nil {
		msg := fmt.Sprintf("response httpstatus:%d, body: %s", rsp.StatusCode, string(b))
		err = errors.WithMessage(err, msg)
		return "", err
	}
	if rsp.StatusCode != http.StatusOK {
		err := errors.Errorf("response httpstatus:%d, body: %s", rsp.StatusCode, string(b))
		return "", err
	}

	rspData := ResponseData{
		HttpStatus:  strconv.Itoa(rsp.StatusCode),
		Header:      rsp.Header,
		Cookies:     rsp.Cookies(),
		RequestData: reqData,
	}
	rspData.Body = string(b)
	jsonByte, err := json.Marshal(rspData)
	if err != nil {
		return "", err
	}
	out = string(jsonByte)
	return out, nil
}

func ReadRequest(httpRaw string) (req *http.Request, err error) {
	httpRaw = pkg.TrimSpaces(httpRaw) // （删除前后空格，对于没有body 内容的请求，后面再加上必要的换行）
	if httpRaw == "" {
		err = errors.Errorf("http raw not allow empty")
		return nil, err
	}
	httpRaw = strings.ReplaceAll(httpRaw, "\r\n", "\n") // 统一换行符
	// 插入body长度头部信息
	bodyIndex := strings.Index(httpRaw, tormfunc.HTTP_HEAD_BODY_DELIM)
	formatHttpRaw := httpRaw
	if bodyIndex > 0 {
		headerRaw := strings.TrimSpace(httpRaw[:bodyIndex])
		bodyRaw := httpRaw[bodyIndex+len(tormfunc.HTTP_HEAD_BODY_DELIM):]
		bodyLen := len(bodyRaw)
		formatHttpRaw = fmt.Sprintf("%s%sContent-Length: %d%s%s", headerRaw, tormfunc.EOF, bodyLen, tormfunc.HTTP_HEAD_BODY_DELIM, bodyRaw)
	} else {
		// 如果没有请求体，则原始字符后面必须保留一个换行符
		formatHttpRaw = fmt.Sprintf("%s%s", formatHttpRaw, tormfunc.HTTP_HEAD_BODY_DELIM)
	}

	buf := bufio.NewReader(strings.NewReader(formatHttpRaw))
	req, err = http.ReadRequest(buf)
	if err != nil {
		return
	}
	if req.URL.Scheme == "" {
		queryPre := ""
		if req.URL.RawQuery != "" {
			queryPre = "?"
		}
		req.RequestURI = fmt.Sprintf("http://%s%s%s%s", req.Host, req.URL.Path, queryPre, req.URL.RawQuery)
	}

	return
}

func Request2RequestData(req *http.Request) (requestDTO *RequestDTO, err error) {
	requestDTO = &RequestDTO{}
	bodyReader, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	bodyByte, err := io.ReadAll(bodyReader)
	if err != nil {
		return
	}
	req.Header.Del("Content-Length")
	requestDTO = &RequestDTO{
		URL:     req.URL.String(),
		Method:  req.Method,
		Header:  req.Header,
		Cookies: req.Cookies(),
		Body:    string(bodyByte),
	}

	return
}
