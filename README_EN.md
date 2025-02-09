# RestGo

English | [简体中文](README.md)

RestGo is a simple, flexible, and powerful Go HTTP client library. It provides a fluent chain-style API that makes building and sending HTTP requests simpler and more elegant.

## Features

- 🚀 Chain-style calls with clean and elegant API design
- 🔄 Built-in retry mechanism with customizable retry strategies
- 🎯 Support for multiple request methods (GET, POST, PUT, DELETE)
- 📦 Support for various data formats (JSON, Form Data, x-www-form-urlencoded)
- 🔍 Automatic request/response serialization/deserialization
- ⏱️ Timeout control and context management
- 🛠️ Customizable HTTP client configuration
- 📝 Curl command generation for debugging

## Installation

```bash
go get github.com/xiao-ren-wu/restgo
```

## Quick Start

### Basic GET Request

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
    log.Printf("Request failed: %v\n", err)
    return
}

fmt.Printf("Status code: %d\n", response.StatusCode())
fmt.Printf("Response body: %s\n", response.BodyStr())
```

### POST Request with JSON Data

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
        "username": "John",
    }).
    RspUnmarshal(&userResp). // Automatically parse response into struct
    Curl(restgo.ConsolePrint). // Print curl command
    Send(restgo.POST, "http://api.example.com/user/register")
```

### Using Retry Mechanism

```go
response, err := restgo.NewRestGoBuilder().
    BaseUrl("http://api.example.com").
    SendWithRetry(restgo.GET, "/retry/test",
        // Define retry condition
        func(respW restgo.Response, err error) error {
            if err != nil {
                return err
            }
            if respW.StatusCode() != 200 {
                return fmt.Errorf("Server error: %d", respW.StatusCode())
            }
            return nil
        },
        // Retry configuration
        retry.Attempts(3),
        retry.Delay(time.Second),
        retry.OnRetry(func(n uint, err error) {
            log.Printf("Retry attempt %d, error: %v\n", n, err)
        }),
    )
```

### Timeout Control

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

response, err := restgo.NewRestGoBuilder().
    CtxSend(ctx, restgo.GET, "http://api.example.com/user/detail/1")
```

## API Documentation

### RestGoBuilder

`RestGoBuilder` is the main interface for building HTTP requests, providing the following methods:

- `NewRestGoBuilder()` - Create a new builder instance
- `BaseUrl(url string)` - Set base URL
- `Headers(headers map[string]string)` - Set request headers
- `Query(queryVal map[string]string)` - Set URL query parameters
- `ContentType(contentType ContentType)` - Set content type
- `Payload(body interface{})` - Set request body
- `RspUnmarshal(v interface{})` - Set response deserialization target
- `Curl(curlConsumer func(curl string))` - Set curl command output function
- `Send(method HttpMethod, url string)` - Send request
- `CtxSend(ctx context.Context, method HttpMethod, url string)` - Send request with context
- `SendWithRetry(method HttpMethod, url string, retryCondition func(Response, error) error, options ...retry.Option)` - Send request with retry

### Response Interface

The `Response` interface provides the following methods to access response data:

- `StatusCode() int` - Get HTTP status code
- `BodyStr() string` - Get response body as string
- `Body() []byte` - Get response body as byte array
- `BodyUnmarshal(v interface{}) error` - Deserialize response body into struct
- `Header(key string) string` - Get response header

## Advanced Features

### Custom Retry Strategy

RestGo uses the [retry-go](https://github.com/avast/retry-go) library for retry mechanism, supporting the following configurations:

- `retry.Attempts(uint)` - Set retry attempts
- `retry.Delay(time.Duration)` - Set retry delay
- `retry.OnRetry(func(n uint, err error))` - Set retry callback
- `retry.RetryIf(func(error) bool)` - Set retry condition

### File Upload

```go
response, err := restgo.NewRestGoBuilder().
    ContentType(restgo.FormData).
    File("file", "path/to/file.txt").
    Payload(map[string]string{
        "user": "erik",
    }).
    Send("POST", "http://api.example.com/upload")
```

## Contributing

Contributions and suggestions are welcome! Please follow these steps:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.