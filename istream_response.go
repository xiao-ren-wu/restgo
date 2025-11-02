package restgo

import "net/http"

type IStreamResponse struct {
	response *http.Response
}

func (sr *IStreamResponse) Header(key string) string {
	if sr == nil || sr.response == nil {
		return ""
	}
	return sr.response.Header.Get(key)
}

func (sr *IStreamResponse) Status() string {
	if sr == nil || sr.response == nil {
		return ""
	}
	return sr.response.Status
}

func (sr *IStreamResponse) StatusCode() int {
	if sr == nil || sr.response == nil {
		return 0
	}
	return sr.response.StatusCode
}

func (sr *IStreamResponse) Proto() string {
	if sr == nil || sr.response == nil {
		return ""
	}
	return sr.response.Proto
}

func (sr *IStreamResponse) ProtoMajor() int {
	if sr == nil || sr.response == nil {
		return 0
	}
	return sr.response.ProtoMajor
}

func (sr *IStreamResponse) ProtoMinor() int {
	if sr == nil || sr.response == nil {
		return 0
	}
	return sr.response.ProtoMinor
}
