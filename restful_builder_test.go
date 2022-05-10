package restgo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"testing"
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
	writer.Write([]byte(fmt.Sprintf("{\"code\":0,\"data\":\"%s\"}", string(bytes))))
}

func startHttpServer() (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/register", userRegister)
	mux.HandleFunc("/user/detail/", findById)
	mux.HandleFunc("/user/update", updateByFormEncoded)
	s := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	go s.ListenAndServe()
	return s, nil
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
	respWrapper, err := NewHttpBuilder().
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
	respWrapper, err := NewHttpBuilder().
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
	respWrapper, err := NewHttpBuilder().
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
	respWrapper, err := NewHttpBuilder().
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
