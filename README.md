# RestGo

[English](README_EN.md) | 简体中文

RestGo 是一个简单、灵活且功能强大的 Go HTTP 客户端库。它提供了流式的链式调用方式，让 HTTP 请求的构建和发送变得更加简单和优雅。

## 特性

- 🚀 链式调用，简洁优雅的 API 设计
- 🔄 内置重试机制，支持自定义重试策略
- 🎯 支持多种请求方式（GET、POST、PUT、DELETE）
- 📦 支持多种数据格式（JSON、Form Data、x-www-form-urlencoded）
- 🔍 支持请求和响应的自动序列化/反序列化
- ⏱️ 支持超时控制和上下文管理
- 🛠️ 支持自定义 HTTP 客户端配置
- 📝 支持生成 curl 命令用于调试

## 安装

```bash
go get github.com/xiao-ren-wu/restgo
```

## 快速开始

### 基础 GET 请求

```go
response, err := restgo.NewRestGoBuilder().
    BaseUrl("http://api.example.com").
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

fmt.Printf("状态码: %d\n", response.StatusCode())
fmt.Printf("响应体: %s\n", response.BodyStr())
```

### POST 请求发送 JSON 数据

```go
type UserResponse struct {
    Code int `json:"code"`
    Data struct {
        Username string `json:"username"`
        UserId   int    `json:"user_id"`
    } `json:"data"`
}

var userResp UserResponse
response, err := restgo.NewRestGoBuilder().
    ContentType(restgo.ApplicationJson).
    Payload(map[string]interface{}{
        "user_id":  5,
        "username": "张三",
    }).
    RspUnmarshal(&userResp). // 自动解析响应到结构体
    Curl(restgo.ConsolePrint). // 打印 curl 命令
    Send(restgo.POST, "http://api.example.com/user/register")
```

### 使用重试机制

```go
response, err := restgo.NewRestGoBuilder().
    BaseUrl("http://api.example.com").
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
```

### 超时控制

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

response, err := restgo.NewRestGoBuilder().
    CtxSend(ctx, restgo.GET, "http://api.example.com/user/detail/1")
```

## API 文档

### RestGoBuilder

`RestGoBuilder` 是构建 HTTP 请求的主要接口，提供以下方法：

- `NewRestGoBuilder()` - 创建新的构建器实例
- `BaseUrl(url string)` - 设置基础 URL
- `Headers(headers map[string]string)` - 设置请求头
- `Query(queryVal map[string]string)` - 设置 URL 查询参数
- `ContentType(contentType ContentType)` - 设置内容类型
- `Payload(body interface{})` - 设置请求体
- `RspUnmarshal(v interface{})` - 设置响应体反序列化目标
- `Curl(curlConsumer func(curl string))` - 设置 curl 命令输出函数
- `Send(method HttpMethod, url string)` - 发送请求
- `CtxSend(ctx context.Context, method HttpMethod, url string)` - 发送带上下文的请求
- `SendWithRetry(method HttpMethod, url string, retryCondition func(Response, error) error, options ...retry.Option)` - 发送带重试的请求

### Response 接口

`Response` 接口提供以下方法访问响应数据：

- `StatusCode() int` - 获取 HTTP 状态码
- `BodyStr() string` - 获取响应体字符串
- `Body() []byte` - 获取响应体字节数组
- `BodyUnmarshal(v interface{}) error` - 将响应体反序列化到结构体
- `Header(key string) string` - 获取响应头

## 高级特性

### 自定义重试策略

RestGo 使用 [retry-go](https://github.com/avast/retry-go) 库实现重试机制，支持以下配置：

- `retry.Attempts(uint)` - 设置重试次数
- `retry.Delay(time.Duration)` - 设置重试间隔
- `retry.OnRetry(func(n uint, err error))` - 设置重试回调
- `retry.RetryIf(func(error) bool)` - 设置重试条件

### 文件上传

```go
response, err := restgo.NewRestGoBuilder().
    ContentType(restgo.FormData).
    File("file", "path/to/file.txt").
    Payload(map[string]string{
        "user": "erik",
    }).
    Send("POST", "http://api.example.com/upload")
```

## 贡献指南

欢迎贡献代码和提出建议！请遵循以下步骤：

1. Fork 本仓库
2. 创建您的特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交您的更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开一个 Pull Request

## 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。