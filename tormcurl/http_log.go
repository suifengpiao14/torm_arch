package tormcurl

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/suifengpiao14/logchan/v2"
)

const (
	HttpLogInfoName LogName = "HttpLogInfo"
)

type HttpLogInfo struct {
	Name           string      `json:"name"`
	Method         string      `json:"method"`
	Url            string      `json:"url"`
	RequestHeader  http.Header `json:"requestHeader"`
	RequestBody    string      `json:"requestBody"`
	ResponseHeader http.Header `json:"responseHeader"`
	ResponseBody   string      `json:"responseBody"`
	CurlCmd        string      `json:"curlCmd"`
	Err            error
	logchan.EmptyLogInfo
}

func (h *HttpLogInfo) GetName() (logName logchan.LogName) {
	return HttpLogInfoName
}

func (h *HttpLogInfo) Error() (err error) {
	return err
}
func (h *HttpLogInfo) BeforSend() {
	h.CurlCmd, _ = h.CURLCli() // 此处的err不能影响业务error
}

//DefaultPrintHttpLogInfo 默认日志打印函数
func DefaultPrintHttpLogInfo(logInfo logchan.LogInforInterface, typeName LogName, err error) {
	if typeName != HttpLogInfoName {
		return
	}
	httpLogInfo, ok := logInfo.(*HttpLogInfo)
	if !ok {
		return
	}
	if err != nil {
		fmt.Fprintf(logchan.LogWriter, "loginInfo:%s,error:%s", httpLogInfo.GetName(), err.Error())
		return
	}
	curlcmd, _ := httpLogInfo.CURLCli()
	fmt.Fprintf(logchan.LogWriter, "curl:%s", curlcmd)
}

// CURLCli 生成curl 命令
func (h HttpLogInfo) CURLCli() (curlCli string, err error) {
	var w bytes.Buffer
	w.WriteString(fmt.Sprintf("curl -X%s", strings.ToUpper(h.Method)))
	for k, v := range h.RequestHeader {
		w.WriteString(fmt.Sprintf(` -H '%s:%v'`, k, v))
	}
	if h.RequestBody != "" {
		w.WriteString(fmt.Sprintf(` -d '%s'`, h.RequestBody))
	}
	w.WriteString(fmt.Sprintf(`'%s'`, h.Url))
	curlCli = w.String()
	return curlCli, nil
}
