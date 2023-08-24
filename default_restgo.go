package restgo

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptrace"
	"time"
)

var client *http.Client

// 配置参考：https://xujiahua.github.io/posts/20200723-golang-http-reuse/
func init() {
	client = &http.Client{
		Timeout: time.Duration(15) * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   512,
			MaxConnsPerHost:       512,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

type defaultRestGo struct {
	client *http.Client
	trace  *httptrace.ClientTrace
}

var defaultRestGoInstance RestGo = NewDefaultRestGo(client, nil)

func NewDefaultRestGo(cli *http.Client, trace *httptrace.ClientTrace) *defaultRestGo {
	return &defaultRestGo{client: cli, trace: trace}
}

func (d *defaultRestGo) Do(ctx context.Context, url string, method string,
	body *bytes.Buffer, contentType string, headers map[string]string) (Response, error) {
	var rsp *http.Response
	var err error
	var req *http.Request
	if d.trace != nil {
		ctx = httptrace.WithClientTrace(ctx, d.trace)
	}
	req, err = http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", contentType)

	if headers != nil {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}
	rsp, err = d.client.Do(req)

	if err != nil {
		return nil, err
	}

	defer rsp.Body.Close()

	bodyBytes, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}
	return &IResponse{
		respBody: bodyBytes,
		response: rsp,
	}, nil
}
