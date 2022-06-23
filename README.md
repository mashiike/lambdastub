# lambdastub

[![Documentation](https://godoc.org/github.com/mashiike/lambdastub?status.svg)](https://godoc.org/github.com/mashiike/lambdastub)
![GitHub go.mod Go version (branch)](https://img.shields.io/github/go-mod/go-version/mashiike/lambdastub)
![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/mashiike/lambdastub)
![Github Actions test](https://github.com/mashiike/lambdastub/workflows/Test/badge.svg?branch=main)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/mashiike/lambdastub/blob/master/LICENSE)

Lambda Stub API Server for [`github.com/aws/aws-lambda-go/lambda`](https://github.com/aws/aws-lambda-go/)

## Usage 

```go
func main() {
	handler := func() (interface{}, error) {
		return "hello world!?", nil
	}

	mux, err := lambdastub.New(
		lambdastub.WithInvokeEndpoint(map[string]interface{}{
			"HelloWorldFunction": handler,
		}),
	)
    if err != nil {
        log.Fatal(err)
    }
    http.ListenAndServe(":8080", mux)
}
```

```shell
$ go run main.go 
$ aws lambda --endpoint http://127.0.0.1:8080 invoke --function-name HelloWorldFunction
```

## LICENSE

MIT License

Copyright (c) 2022 IKEDA Masashi
