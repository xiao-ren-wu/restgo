package restgo

import "errors"

type EmptyResponse struct {
	rsp any
}

func (e *EmptyResponse) Header(key string) string {
	return ""
}

func (e *EmptyResponse) BodyStr() string {
	return ""
}

func (e *EmptyResponse) Body() []byte {
	return nil
}

func (e *EmptyResponse) BodyUnmarshal(v interface{}) error {
	return nil
}

func (e *EmptyResponse) Status() string {
	return ""
}

func (e *EmptyResponse) StatusCode() int {
	return 0
}

func (e *EmptyResponse) Proto() string {
	return ""
}

func (e *EmptyResponse) ProtoMajor() int {
	return 0
}

func (e *EmptyResponse) ProtoMinor() int {
	return 0
}

func (e *EmptyResponse) Rsp() (any, error) {
	if e.rsp == nil {
		return nil, errors.New("rsp struct not set")
	}
	return e.rsp, nil
}
