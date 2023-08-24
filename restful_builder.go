package restgo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/avast/retry-go"
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

var ConsolePrint = func(curl string) {
	fmt.Println(curl)
}

type Builder struct {
	body             interface{}
	contentType      ContentType
	headers          map[string]string
	fileKey          string
	filePath         string
	queryVal         map[string]string
	pathVal          map[string]string
	curlConsumerFunc func(string)
	baseURL          string
	bodyPayload      []byte
	rsp              interface{}
	formFileInfo     *formFileInfo
	restGo           RestGo
}

type formFileInfo struct {
	Reader   io.Reader
	FileKey  string
	FileName string
}

func NewRestGoBuilder() *Builder {
	return &Builder{
		contentType: "application/json",
		restGo:      defaultRestGoInstance,
	}
}

func (builder *Builder) ContentType(contentType ContentType) *Builder {
	builder.contentType = contentType
	return builder
}

func (builder *Builder) Headers(headers map[string]string, conditionFuncs ...ConditionFunc) *Builder {
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
func (builder *Builder) HeaderFunc(hf SFunc) *Builder {
	builder.headers = hf()
	return builder
}

// PayloadFunc 通过函数计算需要传递的payload
func (builder *Builder) PayloadFunc(pf AnyFunc) *Builder {
	builder.body = pf()
	return builder
}

// Payload 请求载荷，根据设置的Content-Type确定最终发送形式
func (builder *Builder) Payload(body interface{}) *Builder {
	builder.body = body
	return builder
}

// BodyPayload 不做任何格式化，直接作为body传递
func (builder *Builder) BodyPayload(body []byte) *Builder {
	builder.bodyPayload = body
	return builder
}

// RspUnmarshal 对响应体进行反序列化，响应体为JSON形式
// 如果只关心响应体序列化到结构体的结果，通过设置这个可以减少代码的编写量
func (builder *Builder) RspUnmarshal(v interface{}) *Builder {
	builder.rsp = v
	return builder
}

func (builder *Builder) File(key, path string) *Builder {
	builder.fileKey = key
	builder.filePath = path
	return builder
}

// FileReader 另外一种上传文件的方式，手动指定reader，如果同时设置调用了 File 方法，那么 File 方法中的参数会优先
func (builder *Builder) FileReader(key, filename string, reader io.Reader) *Builder {
	builder.formFileInfo = &formFileInfo{
		Reader:   reader,
		FileKey:  key,
		FileName: filename,
	}
	return builder
}

// Query URL拼接参数
func (builder *Builder) Query(queryVal map[string]string) *Builder {
	builder.queryVal = queryVal
	return builder
}

// PathVariable 路径参数
func (builder *Builder) PathVariable(pathVal map[string]string) *Builder {
	builder.pathVal = pathVal
	return builder
}

func (builder *Builder) Send(method HttpMethod, url string) (Response, error) {
	return builder.CtxSend(context.Background(), method, url)
}

func (builder *Builder) Curl(curlConsumer func(curl string)) *Builder {
	builder.curlConsumerFunc = curlConsumer
	return builder
}

func (builder *Builder) BaseUrl(baseURL string) *Builder {
	builder.baseURL = baseURL
	return builder
}

func (builder *Builder) generatePayloadAndContentType() (payload *bytes.Buffer, contentType string, bodyCurl string, err error) {
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

func (builder *Builder) generateFormDataFileWriter() (*bytes.Buffer, string, string, error) {

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

func (builder *Builder) generateJsonWriter() (payload *bytes.Buffer, contentType string, bodyCurl string, err error) {
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

func (builder *Builder) saveTmpFile() (string, error) {
	rsp, err := http.Get(builder.filePath) // ignore_security_alert
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()

	bodyBytes, err := io.ReadAll(rsp.Body)
	if err != nil {
		return "", err
	}
	wrapper := &IResponse{
		respBody: bodyBytes,
		response: rsp,
	}
	contentType := http.DetectContentType(wrapper.Body())
	suffix := strings.Split(contentType, "/")[1]

	// 生成临时文件前缀，防止并发上传文件名称冲突问题
	filePrefix := fmt.Sprintf("%d_%d", time.Now().UnixNano(), rand.Int63())
	tmpFile := fmt.Sprintf(tmpFile, filePrefix, suffix)
	err = writeFile(wrapper.Body(), tmpFile)
	if err != nil {
		return "", err
	}
	return tmpFile, nil
}

func (builder *Builder) generateOctetStreamWriter() (*bytes.Buffer, string, string, error) {
	if bytePayload, ok := builder.body.([]byte); ok {
		payload := bytes.NewBuffer(bytePayload)
		contentType := string(builder.contentType)
		return payload, contentType, "raw", nil
	}
	return nil, "", "", fmt.Errorf("body convert bytes failed")
}

func (builder *Builder) generateQuery() string {
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

func (builder *Builder) setPathVariable(url string) (string, error) {
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

func (builder *Builder) generateFormDataEncodedWriter() (*bytes.Buffer, string, string, error) {
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

func (builder *Builder) generateFormDataEncodedPayloadCurl(param map[string]string) string {
	if builder.curlConsumerFunc == nil {
		return ""
	}
	var buf strings.Builder
	for k, v := range param {
		buf.WriteString(fmt.Sprintf("--data-urlencode '%s=%s' \\\n", k, v))
	}
	return buf.String()
}

func (builder *Builder) generateJsonPayloadCurl(reqJson string) string {
	if builder.curlConsumerFunc == nil {
		return ""
	}
	return fmt.Sprintf("--data-raw '%s'", reqJson)
}

func (builder *Builder) generateFormDataPayloadCurl(param map[string]string) string {
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

func (builder *Builder) generateCurl(headers map[string]string, contentType, payload string, url string, method HttpMethod) string {
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

func (builder *Builder) CustomRestGo(customRestGo RestGo) *Builder {
	builder.restGo = customRestGo
	return builder
}

func (builder *Builder) CtxSend(ctx context.Context, method HttpMethod, url string) (Response, error) {
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

	var respW Response
	respW, err = builder.restGo.Do(ctx, url, string(method), body, contentType, builder.headers)
	if err != nil {
		return nil, err
	}

	if builder.rsp != nil {
		if err = json.Unmarshal(respW.Body(), builder.rsp); err != nil {
			return nil, err
		}
	}
	return respW, nil
}

func (builder *Builder) SendWithRetry(method HttpMethod, url string, resF func(respW Response, err error) error, ops ...retry.Option) (Response, error) {
	var respW Response
	var err error
	err = retry.Do(func() error {
		respW, err = builder.Send(method, url)
		return resF(respW, err)
	}, ops...)
	return respW, err
}

func (builder *Builder) CtxSendWithRetry(ctx context.Context, method HttpMethod, url string, resF func(respW Response, err error) error, ops ...retry.Option) (Response, error) {
	var respW Response
	var err error
	err = retry.Do(func() error {
		respW, err = builder.CtxSend(ctx, method, url)
		return resF(respW, err)
	}, ops...)
	return respW, err
}
