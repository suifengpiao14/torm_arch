package tormcurl

import (
	"fmt"
	"io"
	"net/http"

	"github.com/suifengpiao14/logchan/v2"
	"moul.io/http2curl"
)

const (
	LogInfoNameHttp LogName = "HttpLogInfo"
)

//LogInfoHttp 发送日志，只需填写 Request(GetRequest),Response 和RequestBody,其余字段会在BeforSend自动填充
type LogInfoHttp struct {
	Name           string         `json:"name"`
	Request        *http.Request  `json:"-"`
	Response       *http.Response `json:"-"`
	Method         string         `json:"method"`
	Url            string         `json:"url"`
	RequestHeader  http.Header    `json:"requestHeader"`
	RequestBody    string         `json:"requestBody"`
	ResponseHeader http.Header    `json:"responseHeader"`
	ResponseBody   string         `json:"responseBody"`
	CurlCmd        string         `json:"curlCmd"`
	Err            error
	GetRequest     func() (request *http.Request) //go-resty/resty/v2 RawRequest 一开始为空，提供函数延迟实现
	logchan.EmptyLogInfo
}

func (h *LogInfoHttp) GetName() (logName logchan.LogName) {
	return LogInfoNameHttp
}

func (h *LogInfoHttp) Error() (err error) {
	return err
}

// 简化发送方赋值
func (h *LogInfoHttp) BeforSend() {
	if h.GetRequest != nil {
		h.Request = h.GetRequest() // 优先使用延迟获取
	}
	req := h.Request
	resp := h.Response
	if req == nil && resp != nil && resp.Request != nil {
		h.Request = resp.Request
		req = resp.Request
	}
	if req != nil {
		var requestBody []byte
		bodyReader, _ := req.GetBody()
		if bodyReader != nil {
			requestBody, _ = io.ReadAll(bodyReader)

		}
		h.Method = req.Method
		h.Url = req.URL.String()
		h.RequestBody = string(requestBody)
		h.RequestHeader = req.Header.Clone()
		curlCommand, err := http2curl.GetCurlCommand(h.Request)
		if err != nil {
			h.CurlCmd = curlCommand.String()
		}
	}

	if resp != nil {
		if resp.Body != nil {
			responseBody, _ := io.ReadAll(resp.Body)
			h.ResponseBody = string(responseBody)
		}
		h.ResponseHeader = resp.Header.Clone()
	}
}

//DefaultPrintHttpLogInfo 默认日志打印函数
func DefaultPrintHttpLogInfo(logInfo logchan.LogInforInterface, typeName logchan.LogName, err error) {
	if typeName != LogInfoNameHttp {
		return
	}
	httpLogInfo, ok := logInfo.(*LogInfoHttp)
	if !ok {
		return
	}
	if err != nil {
		fmt.Fprintf(logchan.LogWriter, "loginInfo:%s,error:%s\n", httpLogInfo.GetName(), err.Error())
		return
	}
	fmt.Fprintf(logchan.LogWriter, "curl:%s\n", httpLogInfo.CurlCmd)
}
