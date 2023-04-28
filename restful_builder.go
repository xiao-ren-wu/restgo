package restgo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/avast/retry-go"
	"io"
	"io/ioutil"
	"math/rand"
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

type Map map[string]interface{}
type SMap map[string]string
type SFunc func() map[string]string
type AnyFunc func() interface{}
type ConditionFunc func() (string, string)

const (
	tmpFile = "tmp/resource_%s.%s"

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
			MaxIdleConnsPerHost:   512,
			MaxConnsPerHost:       512,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

type httpBuilder struct {
	body             interface{}
	contentType      ContentType
	headers          map[string]string
	fileKey          string
	filePath         string
	queryVal         map[string]string
	pathVal          map[string]string
	curlConsumerFunc func(string)
	baseURL          string
	client           *http.Client
	bodyPayload      []byte
	rsp              interface{}
	formFileInfo     *formFileInfo
}

type formFileInfo struct {
	Reader   io.Reader
	FileKey  string
	FileName string
}

type RespWrapper struct {
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

func (builder *httpBuilder) Headers(headers map[string]string, conditionFuncs ...ConditionFunc) *httpBuilder {
	builder.headers = headers
	if builder.headers == nil {
		builder.headers = make(map[string]string)
	}
	for _, conditionFunc := range conditionFuncs {
		if k, v := conditionFunc(); k != "" && v != "" {
			builder.headers[k] = v
		}
	}
	return builder
}

// HeaderFunc 通过函数计算需要传递的header选项
func (builder *httpBuilder) HeaderFunc(hf SFunc) *httpBuilder {
	builder.headers = hf()
	return builder
}

// PayloadFunc 通过函数计算需要传递的payload
func (builder *httpBuilder) PayloadFunc(pf AnyFunc) *httpBuilder {
	builder.body = pf()
	return builder
}

// Payload 请求载荷，根据设置的Content-Type确定最终发送形式
func (builder *httpBuilder) Payload(body interface{}) *httpBuilder {
	builder.body = body
	return builder
}

// BodyPayload 不做任何格式化，直接作为body传递
func (builder *httpBuilder) BodyPayload(body []byte) *httpBuilder {
	builder.bodyPayload = body
	return builder
}

// RspUnmarshal 对响应体进行反序列化，响应体为JSON形式
// 如果只关心响应体序列化到结构体的结果，通过设置这个可以减少代码的编写量
func (builder *httpBuilder) RspUnmarshal(v interface{}) *httpBuilder {
	builder.rsp = v
	return builder
}

func (builder *httpBuilder) File(key, path string) *httpBuilder {
	builder.fileKey = key
	builder.filePath = path
	return builder
}

// FileReader 另外一种上传文件的方式，手动指定reader，如果同时设置调用了 File 方法，那么 File 方法中的参数会优先
func (builder *httpBuilder) FileReader(key, filename string, reader io.Reader) *httpBuilder {
	builder.formFileInfo = &formFileInfo{
		Reader:   reader,
		FileKey:  key,
		FileName: filename,
	}
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

func (builder *httpBuilder) SetClient(client *http.Client) *httpBuilder {
	builder.client = client
	return builder
}

func (builder *httpBuilder) Send(method HttpMethod, url string) (*RespWrapper, error) {
	// 避免传值传的不是标准的请求方法导致请求错误
	method = HttpMethod(strings.ToUpper(string(method)))

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
	if builder.baseURL != "" {
		url = fmt.Sprintf("%s%s", builder.baseURL, url)
	}

	if body == nil {
		body = bytes.NewBufferString("")
	}

	if builder.curlConsumerFunc != nil {
		curl := builder.generateCurl(builder.headers, contentType, curlPayload, url, method)
		builder.curlConsumerFunc(curl)
	}

	var respW *RespWrapper
	respW, err = builder.sendUsingHTTPClient(url, string(method), body, contentType)
	if err != nil {
		return nil, err
	}
	if builder.rsp != nil {
		if err = json.Unmarshal(respW.respBody, builder.rsp); err != nil {
			return nil, err
		}
	}
	return respW, nil

}

func (wrapper *RespWrapper) Header(key string) string {
	if wrapper != nil {
		if wrapper.response != nil {
			return wrapper.response.Header.Get(key)
		}
	}
	return ""
}

func (wrapper *RespWrapper) BodyStr() string {
	return string(wrapper.respBody)
}

func (wrapper *RespWrapper) Body() []byte {
	return wrapper.respBody
}

func (wrapper *RespWrapper) BodyUnmarshal(v interface{}) error {
	return json.Unmarshal(wrapper.respBody, v)
}

// WriteFile 如果响应是一个文件，可以通过该方法下载文件，path为文件下载路径
func (wrapper *RespWrapper) WriteFile(path string) error {
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

func (builder *httpBuilder) BaseUrl(baseURL string) *httpBuilder {
	builder.baseURL = baseURL
	return builder
}

func (builder *httpBuilder) generatePayloadAndContentType() (payload *bytes.Buffer, contentType string, bodyCurl string, err error) {
	if len(builder.bodyPayload) > 0 {
		return bytes.NewBuffer(builder.bodyPayload),
			string(builder.contentType),
			builder.generateJsonPayloadCurl(string(builder.bodyPayload)), nil
	}

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

	defer func(writer *multipart.Writer) {
		err := writer.Close()
		if err != nil {
			fmt.Printf("close file failed, fail info: %v\n", err)
		}
	}(writer)

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

		formWriter, err := writer.CreateFormFile(builder.fileKey, filepath.Base(builder.filePath))
		if err != nil {
			return nil, "", "", err
		}
		_, err = io.Copy(formWriter, file)
		if err != nil {
			return nil, "", "", err
		}
	} else if builder.formFileInfo != nil {
		formWriter, err := writer.CreateFormFile(builder.formFileInfo.FileKey, filepath.Base(builder.formFileInfo.FileName))
		if err != nil {
			return nil, "", "", err
		}
		if _, err = io.Copy(formWriter, builder.formFileInfo.Reader); err != nil {
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
	} else {
		formDataPayloadCurl = builder.generateFormDataPayloadCurl(nil)
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
	wrapper := &RespWrapper{
		respBody: bodyBytes,
		response: resp,
	}
	contentType := http.DetectContentType(wrapper.respBody)
	suffix := strings.Split(contentType, "/")[1]

	// 生成临时文件前缀，防止并发上传文件名称冲突问题
	filePrefix := fmt.Sprintf("%d_%d", time.Now().UnixNano(), rand.Int63())
	tmpFile := fmt.Sprintf(tmpFile, filePrefix, suffix)
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

func (wrapper *RespWrapper) pathExists(path string) (bool, error) {
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
	} else if builder.formFileInfo != nil {
		buf.WriteString(fmt.Sprintf("--form '%s=@\"%s\"' \\\n", builder.formFileInfo.FileKey, builder.formFileInfo.FileName))
	}
	return buf.String()
}

func (builder *httpBuilder) generateCurl(headers map[string]string, contentType, payload string, url string, method HttpMethod) string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("curl --location --request %s '%s' \\\n", method, url))
	if contentType != "" {
		buf.WriteString(fmt.Sprintf("--header '%s: %s' \\\n", "Content-Type", contentType))
	}
	for k, v := range headers {
		buf.WriteString(fmt.Sprintf("--header '%s: %s' \\\n", k, v))
	}
	buf.WriteString(payload)
	return buf.String()
}

// Status 获取http状态码
func (wrapper *RespWrapper) Status() string {
	return wrapper.response.Status
}

func (wrapper *RespWrapper) StatusCode() int {
	return wrapper.response.StatusCode
}

func (wrapper *RespWrapper) Proto() string {
	return wrapper.response.Proto
}

func (wrapper *RespWrapper) ProtoMajor() int {
	return wrapper.response.ProtoMajor
}

func (wrapper *RespWrapper) ProtoMinor() int {
	return wrapper.response.ProtoMinor
}

type HttpBuilder struct {
	builder *httpBuilder
}

// Build 用于保存生成的 httpBuilder，方便重复发起请求，该 HttpBuilder 是只读的
func (builder *httpBuilder) Build() *HttpBuilder {
	return &HttpBuilder{
		builder: builder,
	}
}

func (builder *httpBuilder) sendUsingHTTPClient(
	url string, method string, body *bytes.Buffer, contentType string,
) (*RespWrapper, error) {
	var resp *http.Response
	var err error
	var req *http.Request
	req, err = http.NewRequest(method, url, body)

	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", contentType)

	if builder.headers != nil {
		for k, v := range builder.headers {
			req.Header.Add(k, v)
		}
	}
	// 设置局部的client使用局部的client发请求
	if builder.client != nil {
		resp, err = builder.client.Do(req)
	} else {
		resp, err = client.Do(req)
	}

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &RespWrapper{
		respBody: bodyBytes,
		response: resp,
	}, nil
}

// ToBuilder 将 HttpBuilder 转换成 httpBuilder
func (hb *HttpBuilder) ToBuilder() *httpBuilder {
	return hb.builder
}

func (hb *HttpBuilder) Send(method HttpMethod, url string) (*RespWrapper, error) {
	return hb.builder.Send(method, url)
}

func (builder *httpBuilder) SendWithRetry(method HttpMethod, url string, resF func(respW *RespWrapper, err error) error, ops ...retry.Option) (*RespWrapper, error) {
	var respW *RespWrapper
	var err error
	err = retry.Do(func() error {
		respW, err = builder.Send(method, url)
		return resF(respW, err)
	}, ops...)
	return respW, err
}
