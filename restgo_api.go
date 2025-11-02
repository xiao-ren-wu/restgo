package restgo

import (
	"bytes"
	"context"
)

type Response interface {
	Header(key string) string
	BodyStr() string
	Body() []byte
	BodyUnmarshal(v interface{}) error
	Status() string
	StatusCode() int
	Proto() string
	ProtoMajor() int
	ProtoMinor() int
}

type RestGo interface {
	Do(ctx context.Context, url string, method string, body *bytes.Buffer, contentType string, headers map[string]string) (Response, error)
}

type StreamResponse interface {
	Header(key string) string
	Status() string
	StatusCode() int
	Proto() string
	ProtoMajor() int
	ProtoMinor() int
}

type StreamRestGo interface {
	DoStream(ctx context.Context, url string, method string, body *bytes.Buffer, contentType string, headers map[string]string, callback func(StreamResponse, string) error) error
}
