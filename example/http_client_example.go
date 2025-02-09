package example

import (
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"log"
	"time"

	"github.com/xiao-ren-wu/restgo"
)

// UserResponse 用于解析响应数据的结构体
type UserResponse struct {
	Code int `json:"code"`
	Data struct {
		Username string `json:"username"`
		UserId   int    `json:"user_id"`
	} `json:"data"`
}

func ExampleHttpRequest() {
	// 1. 基本GET请求示例
	response, err := restgo.NewRestGoBuilder().
		BaseUrl("http://localhost:8080").
		Headers(map[string]string{
			"X-Request-ID": "123456",
			"Authorization": "Bearer token123",
		}).
		Query(map[string]string{
			"id": "1",
		}).
		Send(restgo.GET, "/user/detail")

	if err != nil {
		log.Printf("请求失败: %v\n", err)
		return
	}

	// 打印响应状态码和响应体
	fmt.Printf("状态码: %d\n", response.StatusCode())
	fmt.Printf("响应体: %s\n", response.BodyStr())

	// 2. POST请求示例 - 发送JSON数据
	var userResp UserResponse
	response, err = restgo.NewRestGoBuilder().
		ContentType(restgo.ApplicationJson).
		Payload(map[string]interface{}{
			"user_id":  5,
			"username": "张三",
		}).
		RspUnmarshal(&userResp). // 自动解析响应到结构体
		Curl(restgo.ConsolePrint). // 打印curl命令
		Send(restgo.POST, "http://localhost:8080/user/register")

	if err != nil {
		log.Printf("注册用户失败: %v\n", err)
		return
	}

	// 3. 带重试机制的请求示例
	response, err = restgo.NewRestGoBuilder().
		BaseUrl("http://localhost:8080").
		SendWithRetry(restgo.GET, "/retry/test",
			// 定义重试条件
			func(respW restgo.Response, err error) error {
				if err != nil {
					return err
				}
				if respW.StatusCode() != 200 {
					return fmt.Errorf("服务端错误: %d", respW.StatusCode())
				}
				return nil
			},
			// 重试配置
			retry.Attempts(3),
			retry.Delay(time.Second),
			retry.OnRetry(func(n uint, err error) {
				log.Printf("第%d次重试, 错误: %v\n", n, err)
			}),
		)

	if err != nil {
		log.Printf("重试后仍然失败: %v\n", err)
		return
	}

	// 4. 带上下文的请求示例
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err = restgo.NewRestGoBuilder().
		CtxSend(ctx, restgo.GET, "http://localhost:8080/user/detail/1")

	if err != nil {
		log.Printf("请求超时或失败: %v\n", err)
		return
	}
}