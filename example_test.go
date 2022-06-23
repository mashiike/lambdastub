package lambdastub_test

import (
	"context"
	"fmt"
	"net/http/httptest"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/mashiike/lambdastub"
)

func Example() {
	handler := func() (interface{}, error) {
		return "hello world!?", nil
	}

	mux, err := lambdastub.New(
		lambdastub.WithInvokeEndpoint(map[string]interface{}{
			"HelloWorldFunction": handler,
		}),
	)
	if err != nil {
		fmt.Println("error: ", err)
	}
	server := httptest.NewServer(mux)
	defer server.Close()
	client := lambda.New(
		lambda.Options{
			Credentials: credentials.NewStaticCredentialsProvider("ACCESS_KEY_ID", "SECRET_KEY", "TOKEN"),
			EndpointResolver: lambda.EndpointResolverFunc(func(region string, options lambda.EndpointResolverOptions) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               server.URL,
					PartitionID:       "aws",
					SigningRegion:     "us-west-1",
					HostnameImmutable: true,
				}, nil
			}),
		},
	)
	output, err := client.Invoke(context.Background(), &lambda.InvokeInput{
		FunctionName: aws.String("HelloWorldFunction"),
		Payload:      nil,
	})
	if err != nil {
		fmt.Println("error: ", err)
	}
	fmt.Println(string(output.Payload))
	// output: "hello world!?"
}
