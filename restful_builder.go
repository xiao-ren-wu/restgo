package restgo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 提供"傻瓜式"HTTP客户端工具,用户只需要通过"点点点"的编程风格即可构造一个http请求
// 目前支持 GET/POST 请求
// 支持路径参数以及URL拼接形式
// POST 请求支持:
// - multipart/form-data
// - application/x-www-form-urlencoded
// - application/json
// - binary/octet-stream

type HttpMethod string

type ContentType string

const (
	tmpFile = "tmp/resource.%s"

	GET    = HttpMethod("GET")
	POST   = HttpMethod("POST")
	PUT    = HttpMethod("PUT")
	DELETE = HttpMethod("DELETE")

	ApplicationJson = ContentType("application/json")
	FormData        = ContentType("multipart/form-data")
	FormDataEncoded = ContentType("application/x-www-form-urlencoded")
	OctetStream     = ContentType("binary/octet-stream")
)

var client *http.Client

var ConsolePrint = func(curl string) {
	fmt.Println(curl)
}

// 配置参考：https://xujiahua.github.io/posts/20200723-golang-http-reuse/
func init() {
	client = &http.Client{
		Timeout: time.Duration(15) * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   1,
			MaxConnsPerHost:       1,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

type httpBuilder struct {
	path             string
	pathParams       string
	urlParams        string
	body             interface{}
	formData         *url.Values
	contentType      ContentType
	headers          map[string]string
	fileKey          string
	filePath         string
	queryVal         map[string]string
	pathVal          map[string]string
	curlConsumerFunc func(string)
}

type respWrapper struct {
	respBody []byte
	response *http.Response
}

func NewHttpBuilder() *httpBuilder {
	return &httpBuilder{
		contentType: "application/json",
	}
}

func (builder *httpBuilder) ContentType(contentType ContentType) *httpBuilder {
	builder.contentType = contentType
	return builder
}

func (builder *httpBuilder) Headers(headers map[string]string) *httpBuilder {
	builder.headers = headers
	return builder
}

// Payload 请求载荷，根据设置的Content-Type确定最终发送形式
func (builder *httpBuilder) Payload(body interface{}) *httpBuilder {
	builder.body = body
	return builder
}

func (builder *httpBuilder) File(key, path string) *httpBuilder {
	builder.fileKey = key
	builder.filePath = path
	return builder
}

// Query URL拼接参数
func (builder *httpBuilder) Query(queryVal map[string]string) *httpBuilder {
	builder.queryVal = queryVal
	return builder
}

// PathVariable 路径参数
func (builder *httpBuilder) PathVariable(pathVal map[string]string) *httpBuilder {
	builder.pathVal = pathVal
	return builder
}

func (builder *httpBuilder) Send(method HttpMethod, url string) (*respWrapper, error) {

	body, contentType, curlPayload, err := builder.generatePayloadAndContentType()
	if err != nil {
		return nil, err
	}

	url, err = builder.setPathVariable(url)
	if err != nil {
		return nil, err
	}

	query := builder.generateQuery()
	if query != "" {
		url = fmt.Sprintf("%s?%s", url, query)
	}
	var req *http.Request
	if body == nil {
		body = bytes.NewBufferString("")
	}
	req, err = http.NewRequest(string(method), url, body)

	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", contentType)

	if builder.headers != nil {
		for k, v := range builder.headers {
			req.Header.Add(k, v)
		}
	}

	if builder.curlConsumerFunc != nil {
		curl := builder.generateCurl(builder.headers, contentType, curlPayload, url, method)
		builder.curlConsumerFunc(curl)
	}

	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &respWrapper{
		respBody: bodyBytes,
		response: resp,
	}, nil

}

func (wrapper *respWrapper) Header(key string) string {
	if wrapper != nil {
		if wrapper.response != nil {
			return wrapper.response.Header.Get(key)
		}
	}
	return ""
}

func (wrapper *respWrapper) BodyStr() string {
	return string(wrapper.respBody)
}

func (wrapper *respWrapper) Body() []byte {
	return wrapper.respBody
}

func (wrapper *respWrapper) BodyUnmarshal(v interface{}) error {
	return json.Unmarshal(wrapper.respBody, v)
}

// WriteFile 如果响应是一个文件，可以通过该方法下载文件，path为文件下载路径
func (wrapper *respWrapper) WriteFile(path string) error {
	if path == "" {
		return fmt.Errorf("writefile failed, path must not be emoty")
	}
	dir := path[:strings.LastIndex(path, "/")]
	exists, err := wrapper.pathExists(dir)
	if err != nil {
		return err
	}
	if !exists {
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return ioutil.WriteFile(path, wrapper.respBody, os.ModePerm) // ignore_security_alert
}

func (builder *httpBuilder) Curl(curlConsumer func(curl string)) *httpBuilder {
	builder.curlConsumerFunc = curlConsumer
	return builder
}

func (builder *httpBuilder) generatePayloadAndContentType() (payload *bytes.Buffer, contentType string, bodyCurl string, err error) {

	if builder.fileKey != "" {
		builder.contentType = FormData
	}

	switch builder.contentType {
	case ApplicationJson:
		return builder.generateJsonWriter()
	case FormData:
		return builder.generateFormDataFileWriter()
	case OctetStream:
		return builder.generateOctetStreamWriter()
	case FormDataEncoded:
		return builder.generateFormDataEncodedWriter()
	default:
		return nil, "", "", fmt.Errorf("content-type:[%s] not support", builder.contentType)
	}
}

func (builder *httpBuilder) generateFormDataFileWriter() (*bytes.Buffer, string, string, error) {

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	var err error

	if builder.filePath != "" {
		var tmpFile string

		// 处理下载链接
		if strings.HasPrefix(builder.filePath, "http") {
			tmpFile, err = builder.saveTmpFile()
			if err != nil {
				return nil, "", "", err
			}
		}
		if tmpFile != "" {
			builder.filePath = tmpFile
			defer func(name string) {
				err := os.Remove(name) // ignore_security_alert
				if err != nil {
					fmt.Printf("del tmp file [%s] failed, fail info: %v\n", tmpFile, err)
				}
			}(tmpFile)
		}

		file, err := os.Open(builder.filePath)
		defer func(file *os.File) {
			err := file.Close()
			if err != nil {
				fmt.Printf("close file failed, fail info: %v\n", err)
			}
		}(file)
		if err != nil {
			return nil, "", "", err
		}

		defer func(writer *multipart.Writer) {
			err := writer.Close()
			if err != nil {
				fmt.Printf("close file failed, fail info: %v\n", err)
			}
		}(writer)

		fromWriter, err := writer.CreateFormFile(builder.fileKey, filepath.Base(builder.filePath))
		_, err = io.Copy(fromWriter, file)

		if err != nil {
			return nil, "", "", err
		}
	}
	var formDataPayloadCurl string
	if builder.body != nil {
		var bodybytes []byte
		bodybytes, err = json.Marshal(builder.body)
		param := make(map[string]string)
		err := json.Unmarshal(bodybytes, &param)
		if err != nil {
			return nil, "", "", err
		}
		formDataPayloadCurl = builder.generateFormDataPayloadCurl(param)
		for key, val := range param {
			_ = writer.WriteField(key, val)
		}
	}

	return body, writer.FormDataContentType(), formDataPayloadCurl, nil
}

func (builder *httpBuilder) generateJsonWriter() (payload *bytes.Buffer, contentType string, bodyCurl string, err error) {
	if builder.body == nil {
		return nil, "", "", nil
	}
	jsonBytes, err := json.Marshal(builder.body)
	if err != nil {
		return nil, "", "", err
	}
	payload = bytes.NewBuffer(jsonBytes)
	contentType = string(builder.contentType)
	bodyCurl = builder.generateJsonPayloadCurl(string(jsonBytes))
	return
}

func (builder *httpBuilder) saveTmpFile() (string, error) {
	resp, err := http.Get(builder.filePath) // ignore_security_alert
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	wrapper := &respWrapper{
		respBody: bodyBytes,
		response: resp,
	}
	contentType := http.DetectContentType(wrapper.respBody)
	suffix := strings.Split(contentType, "/")[1]
	tmpFile := fmt.Sprintf(tmpFile, suffix)
	err = wrapper.WriteFile(tmpFile)
	if err != nil {
		return "", err
	}
	return tmpFile, nil
}

func (builder *httpBuilder) generateOctetStreamWriter() (*bytes.Buffer, string, string, error) {
	if bytePayload, ok := builder.body.([]byte); ok {
		payload := bytes.NewBuffer(bytePayload)
		contentType := string(builder.contentType)
		return payload, contentType, "raw", nil
	}
	return nil, "", "", fmt.Errorf("body convert bytes failed")
}

func (wrapper *respWrapper) pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (builder *httpBuilder) generateQuery() string {
	queryVal := builder.queryVal
	if queryVal == nil {
		return ""
	}
	var kvList []string
	for k, v := range queryVal {
		kvList = append(kvList, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(kvList, "&")
}

func (builder *httpBuilder) setPathVariable(url string) (string, error) {
	if builder.pathVal == nil {
		return url, nil
	}
	subPaths := strings.Split(url, "/")
	for idx, subPath := range subPaths {
		if strings.HasPrefix(subPath, ":") {
			key := subPath[1:]
			val, ok := builder.pathVal[key]
			if !ok {
				return "", fmt.Errorf("path val [%s] not set", key)
			}
			subPaths[idx] = val
		}
	}
	return strings.Join(subPaths, "/"), nil
}

func (builder *httpBuilder) generateFormDataEncodedWriter() (*bytes.Buffer, string, string, error) {
	if builder.body == nil {
		return nil, "", "", nil
	}
	data := url.Values{}
	bodyBytes, err := json.Marshal(builder.body)
	param := make(map[string]string)
	err = json.Unmarshal(bodyBytes, &param)
	if err != nil {
		return nil, "", "", err
	}
	for key, val := range param {
		data.Set(key, val)
	}
	var b bytes.Buffer
	b.Write([]byte(data.Encode()))
	return &b, string(FormDataEncoded), builder.generateFormDataEncodedPayloadCurl(param), nil
}

func (builder *httpBuilder) generateFormDataEncodedPayloadCurl(param map[string]string) string {
	if builder.curlConsumerFunc == nil {
		return ""
	}
	var buf strings.Builder
	for k, v := range param {
		buf.WriteString(fmt.Sprintf("--data-urlencode '%s=%s' \\\n", k, v))
	}
	return buf.String()
}

func (builder *httpBuilder) generateJsonPayloadCurl(reqJson string) string {
	if builder.curlConsumerFunc == nil {
		return ""
	}
	return fmt.Sprintf("--data-raw '%s'", reqJson)
}

func (builder *httpBuilder) generateFormDataPayloadCurl(param map[string]string) string {
	if builder.curlConsumerFunc == nil {
		return ""
	}
	var buf strings.Builder
	for k, v := range param {
		buf.WriteString(fmt.Sprintf("--form '%s=\"%s\"' \\\n", k, v))
	}
	if builder.fileKey != "" {
		buf.WriteString(fmt.Sprintf("--form '%s=@\"%s\"' \\\n", builder.fileKey, builder.filePath))
	}
	return buf.String()
}

func (builder *httpBuilder) generateCurl(headers map[string]string, contentType, payload string, url string, method HttpMethod) string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("curl --location --request %s '%s' \\\n", method, url))
	buf.WriteString(fmt.Sprintf("--header '%s: %s' \\\n", "Content-Type", contentType))
	for k, v := range headers {
		buf.WriteString(fmt.Sprintf("--header '%s: %s' \\\n", k, v))
	}
	buf.WriteString(payload)
	return buf.String()
}
