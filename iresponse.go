package restgo

import (
	"encoding/json"
	"net/http"
)

type IResponse struct {
	respBody []byte
	response *http.Response
}

func (wrapper *IResponse) Header(key string) string {
	if wrapper != nil {
		if wrapper.response != nil {
			return wrapper.response.Header.Get(key)
		}
	}
	return ""
}

func (wrapper *IResponse) BodyStr() string {
	return string(wrapper.respBody)
}

func (wrapper *IResponse) Body() []byte {
	return wrapper.respBody
}

func (wrapper *IResponse) BodyUnmarshal(v interface{}) error {
	return json.Unmarshal(wrapper.respBody, v)
}

// Status 获取http状态码
func (wrapper *IResponse) Status() string {
	return wrapper.response.Status
}

func (wrapper *IResponse) StatusCode() int {
	return wrapper.response.StatusCode
}

func (wrapper *IResponse) Proto() string {
	return wrapper.response.Proto
}

func (wrapper *IResponse) ProtoMajor() int {
	return wrapper.response.ProtoMajor
}

func (wrapper *IResponse) ProtoMinor() int {
	return wrapper.response.ProtoMinor
}
