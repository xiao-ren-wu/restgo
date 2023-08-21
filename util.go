package restgo

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// WriteFile 如果响应是一个文件，可以通过该方法下载文件，path为文件下载路径
func writeFile(bodyRaw []byte,path string) error {
	if path == "" {
		return fmt.Errorf("writefile failed, path must not be emoty")
	}
	dir := path[:strings.LastIndex(path, "/")]
	exists, err := pathExists(dir)
	if err != nil {
		return err
	}
	if !exists {
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return ioutil.WriteFile(path, bodyRaw, os.ModePerm) // ignore_security_alert
}


func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
