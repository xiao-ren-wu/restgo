package restgo

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
)

type defaultStreamRestGo struct {
	client *http.Client
	trace  *httptrace.ClientTrace
}

var defaultStreamRestGoInstance StreamRestGo = NewDefaultStreamRestGo(initClient(), nil)

func NewDefaultStreamRestGo(cli *http.Client, trace *httptrace.ClientTrace) *defaultStreamRestGo {
	return &defaultStreamRestGo{client: cli, trace: trace}
}

func (d *defaultStreamRestGo) DoStream(ctx context.Context, url string, method string,
	body *bytes.Buffer, contentType string, headers map[string]string, callback func(resp StreamResponse, rspBody string) error) error {
	var rsp *http.Response
	var err error
	var req *http.Request
	if d.trace != nil {
		ctx = httptrace.WithClientTrace(ctx, d.trace)
	}
	req, err = http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", contentType)
	req.Header.Set("Accept", "text/event-stream")

	if headers != nil {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}
	rsp, err = d.client.Do(req)

	if err != nil {
		return err
	}

	defer rsp.Body.Close()
	var streamResponse = &IStreamResponse{response: rsp}
	return d.streamHandler(rsp, streamResponse, callback)
}

func (d *defaultStreamRestGo) streamHandler(resp *http.Response, streamResponse StreamResponse, callback func(resp StreamResponse, rspBody string) error) error {
	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error: %s, read rsp body failed", resp.Status)
		}
		return fmt.Errorf("error: %s, body: %s\n", resp.Status, body)
	}

	// 创建读取器来逐行读取
	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// 去除首尾空白字符
		line = strings.TrimSpace(line)

		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// 解析 SSE 格式：data: {...}
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// 检查是否结束
			if data == "[DONE]" {
				break
			}
			if err = callback(streamResponse, data); err != nil {
				return err
			}
		}
	}
	return nil
}
