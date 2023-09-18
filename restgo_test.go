package restgo

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/avast/retry-go"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type Person struct {
	Username string `json:"username"`
	UserId   int    `json:"user_id"`
}

var dbMap map[int]*Person

func initDb() {
	dbMap = map[int]*Person{
		1: {
			Username: "Tom",
			UserId:   1,
		},
		2: {
			Username: "Erik",
			UserId:   2,
		},
		3: {
			Username: "Jerry",
			UserId:   3,
		},
		4: {
			Username: "Kite",
			UserId:   4,
		},
	}
}

func userRegister(writer http.ResponseWriter, request *http.Request) {
	bodyBytes, err := ioutil.ReadAll(request.Body)
	if err != nil {
		respErr(writer)
		return
	}
	var req Person
	err = json.Unmarshal(bodyBytes, &req)
	if err != nil {
		respErr(writer)
		return
	}
	dbMap[req.UserId] = &req
	respOk(writer)
}

func findById(writer http.ResponseWriter, request *http.Request) {
	id := request.URL.Query().Get("id")
	if id == "" {
		path := request.URL.Path
		id = path[strings.LastIndex(path, "/")+1:]
	}
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		respErr(writer)
		return
	}
	respOkWithData(writer, dbMap[int(idInt)])
}

func updateByFormEncoded(writer http.ResponseWriter, request *http.Request) {
	id := request.PostFormValue("id")
	username := request.PostFormValue("username")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		respErr(writer)
		return
	}
	if person, ok := dbMap[int(idInt)]; ok {
		person.Username = username
	}
	respOk(writer)
}

func respErr(writer http.ResponseWriter) {
	writer.Write([]byte(("{\"code\":-1}")))
}

func respOk(writer http.ResponseWriter) {
	writer.Write([]byte(("{\"code\":0}")))
	writer.Header().Set("logId", "123")
}

func respOkWithData(writer http.ResponseWriter, resp interface{}) {
	bytes, _ := json.Marshal(resp)
	writer.Write([]byte(fmt.Sprintf("{\"code\":0,\"data\":%s}", string(bytes))))
}

func startHttpServer() (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/register", userRegister)
	mux.HandleFunc("/user/detail/", findById)
	mux.HandleFunc("/user/update", updateByFormEncoded)
	mux.HandleFunc("/user/upload_cover", uploadUserCover)
	mux.HandleFunc("/retry/test", retryMock)
	mux.HandleFunc("/headers", respHeaders)
	s := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	go s.ListenAndServe()
	return s, nil
}

func respHeaders(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/plain")
	writer.Header().Set("X-Custom-Header", "custom value")
	writer.WriteHeader(http.StatusOK)
}

var cnt = 0

func retryMock(writer http.ResponseWriter, request *http.Request) {
	if cnt < 3 {
		cnt++
		writer.WriteHeader(http.StatusGatewayTimeout)
		respErr(writer)
		return
	}
	respOk(writer)
}

func uploadUserCover(writer http.ResponseWriter, request *http.Request) {
	file, fileHeader, err := request.FormFile("cover")
	if err != nil {
		respErr(writer)
		return
	}
	defer file.Close()
	fileRaw, err := io.ReadAll(file)
	if err != nil {
		respErr(writer)
		return
	}
	respOkWithData(writer, map[string]interface{}{
		"file_name": fileHeader.Filename,
		"file_size": fileHeader.Size,
		"file_raw":  string(fileRaw),
		"username":  request.FormValue("user"),
	})
}

func TestMain(m *testing.M) {
	initDb()
	server, err := startHttpServer()
	if err != nil {
		return
	}
	m.Run()
	server.Close()
}

func TestHttpBuilder_Query(t *testing.T) {
	respWrapper, err := NewRestGoBuilder().
		Query(map[string]string{
			"id": "1",
		}).
		Curl(ConsolePrint).
		Send(GET, "http://localhost:8080/user/detail")
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Log(respWrapper.BodyStr())
	t.Log(respWrapper.Header("logId"))
	t.Log(string(respWrapper.Body()))

	type resp struct {
		Code int `json:"code"`
	}
	var res resp
	respWrapper.BodyUnmarshal(&res)
	t.Log(res.Code)
}

func TestHttpBuilder_PathVariable(t *testing.T) {
	respWrapper, err := NewRestGoBuilder().
		PathVariable(map[string]string{
			"id": "2",
		}).
		Curl(ConsolePrint).
		Send(GET, "http://localhost:8080/user/detail/:id")
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Log(respWrapper.BodyStr())
}

func TestHttpBuilder_Payload_Body(t *testing.T) {
	respWrapper, err := NewRestGoBuilder().
		ContentType(ApplicationJson).
		Payload(map[string]interface{}{
			"user_id":  5,
			"username": "Rose",
		}).
		Curl(ConsolePrint).
		Send(POST, "http://localhost:8080/user/register")
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Log(respWrapper.BodyStr())
}

func TestHttpBuilder_Payload_FromData(t *testing.T) {
	respWrapper, err := NewRestGoBuilder().
		ContentType(FormDataEncoded).
		Payload(map[string]interface{}{
			"id":       "5",
			"username": "Rose",
		}).
		Curl(ConsolePrint).
		Send(POST, "http://localhost:8080/user/update")
	if err != nil {
		t.Fatal(err)
		return
	}
	t.Log(respWrapper.BodyStr())
}

func TestHttpBuilder_BaseUrl(t *testing.T) {
	respWrapper, err := NewRestGoBuilder().
		PathVariable(map[string]string{
			"id": "2",
		}).
		BaseUrl("http://localhost:8080").
		Send(GET, "/user/detail/:id")
	if err != nil {
		t.Fatal(err)
		return
	}
	body := respWrapper.Body()
	t.Log(string(body))
}

func TestHttpBuilder_HeaderFunc(t *testing.T) {
	respWrapper, err := NewRestGoBuilder().
		PathVariable(map[string]string{
			"id": "2",
		}).
		BaseUrl("http://localhost:8080").
		Send(GET, "/user/detail/:id")
	if err != nil {
		t.Fatal(err)
		return
	}
	body := respWrapper.Body()
	t.Log(string(body))
}

func TestHttpBuilder_HeaderFunc2(t *testing.T) {
	respWrapper, err := NewRestGoBuilder().
		Headers(map[string]string{
			"token": "234",
		}, func() (string, string) {
			return "x-tt-env", "boe_feat"
		}).
		BaseUrl("http://localhost:8080").
		Curl(func(curl string) {
			t.Log(curl)
		}).
		Send(GET, "/user/detail/:id")
	if err != nil {
		t.Fatal(err)
		return
	}
	body := respWrapper.Body()
	t.Log(string(body))
}

func TestHttpBuilder_Funcs(t *testing.T) {
	respWrapper, err := NewRestGoBuilder().
		HeaderFunc(func() map[string]string {
			return map[string]string{
				"token":    "234",
				"x-tt-env": "boe_feat",
			}
		}).
		PayloadFunc(func() interface{} {
			return map[string]interface{}{
				"user_id":  5,
				"username": "Rose",
			}
		}).
		Curl(ConsolePrint).
		Send(POST, "http://localhost:8080/user/register")
	if err != nil {
		t.Fatal(err)
		return
	}
	body := respWrapper.Body()
	t.Log(string(body))
}

func TestHttpBuilder_OriginPayload(t *testing.T) {
	respWrapper, err := NewRestGoBuilder().
		BodyPayload(func() []byte {
			marshal, _ := json.Marshal(map[string]interface{}{
				"user_id":  5,
				"username": "Rose",
			})
			return marshal
		}()).
		Curl(ConsolePrint).
		Send(POST, "http://localhost:8080/user/register")
	if err != nil {
		t.Fatal(err)
		return
	}
	body := respWrapper.Body()
	t.Log(string(body))
}

func TestHttpBuilder_BodyMarshal(t *testing.T) {
	type resp struct {
		Code int `json:"code"`
	}
	var res resp
	if _, err := NewRestGoBuilder().
		Payload(map[string]interface{}{
			"user_id":  5,
			"username": "Rose",
		}).
		RspUnmarshal(&res).
		Send(POST, "http://localhost:8080/user/register"); err != nil {
		t.Fatal(err)
		return
	}
	t.Logf("%#v", res)
}

//func TestHttpUsingPSMDiscovery(t *testing.T) {
//	type resp struct {
//		Code int `json:"code"`
//	}
//	var res resp
//	if _, err := NewRestGoBuilder().
//		Payload(map[string]interface{}{
//			"user_id":  5,
//			"username": "Rose",
//		}).
//		Curl(ConsolePrint).
//		RspUnmarshal(&res).
//		UsePSMDiscovery("ic.cp.operation_backend").
//		Send(POST, "/user/register"); err != nil {
//		t.Fatal(err)
//		return
//	}
//	t.Logf("%#v", res)
//}

func TestFromFile(t *testing.T) {
	rsp, err := NewRestGoBuilder().
		ContentType(FormData).
		File("cover", "testdata/formfile.txt").
		Payload(map[string]string{
			"user": "erik",
		}).
		Curl(ConsolePrint).
		Send("POST", "http://localhost:8080/user/upload_cover")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(rsp.BodyStr())
}

func TestUsingCustomerClient(t *testing.T) {
	rsp, err := NewRestGoBuilder().
		ContentType(FormData).
		Headers(nil).
		File("cover", "testdata/formfile.txt").
		Payload(map[string]string{
			"user": "erik",
		}).
		Curl(ConsolePrint).
		Send("POST", "http://localhost:8080/user/upload_cover")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(rsp.BodyStr())
}

func TestFormFileReader(t *testing.T) {
	file, err := os.Open("testdata/formfile.txt")
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Printf("close file failed, fail info: %v\n", err)
		}
	}(file)
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(file)
	rsp, err := NewRestGoBuilder().
		ContentType(FormData).
		Payload(map[string]string{
			"user": "erik",
		}).
		Curl(ConsolePrint).
		FileReader("cover", "erik-test.png", reader).
		Send("POST", "http://localhost:8080/user/upload_cover")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(rsp.BodyStr())
}

func TestHttpBuilder_SendWithRetry(t *testing.T) {
	retryCondition := func(respW Response, err error) error {
		if err != nil {
			return err
		}
		if respW.StatusCode() != 200 {
			return fmt.Errorf("http status code: %v", respW.StatusCode())
		}
		return nil
	}

	rsp, err := NewRestGoBuilder().
		BaseUrl("http://localhost:8080").
		SendWithRetry(GET, "/retry/test", retryCondition,
			retry.Attempts(10),
			retry.Delay(3*time.Second),
			retry.LastErrorOnly(true),
			retry.OnRetry(func(n uint, err error) {
				t.Logf("retry times:%v, reason: %v", n, err)
			}),
			//retry.RetryIf(func(err error) bool {
			//	return false
			//}),
		)

	if err != nil {
		t.Fatal(err)
	}
	t.Log(rsp.BodyStr())
}
