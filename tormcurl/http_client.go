package tormcurl

import (
	"io"
	"net/http"

	"github.com/suifengpiao14/logchan/v2"
)

//DoHttpWithLogInfo 包装http请求，记录日志,do 报错后 ,resp 可能为空，所有将req调整为入参，方便记录请求数据
func DoHttpWithLogInfo(req *http.Request, do func() (resp *http.Response, responseBody []byte, err error)) {
	var requestBody []byte
	var err error
	bodyReader, _ := req.GetBody()
	if bodyReader != nil {
		requestBody, err = io.ReadAll(bodyReader)
		if err != nil {
			return
		}
	}
	httpLogInfo := LogInfoHttp{
		Method:        req.Method,
		Url:           req.URL.String(),
		RequestBody:   string(requestBody),
		RequestHeader: req.Header.Clone(),
	}

	defer func() {
		logchan.SendLogInfo(&httpLogInfo)
	}()

	resp, responseBody, err := do()
	httpLogInfo.Err = err
	httpLogInfo.ResponseBody = string(responseBody)
	if resp != nil {
		httpLogInfo.ResponseHeader = resp.Header.Clone()
	}

}
