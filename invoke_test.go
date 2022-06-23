package lambdastub_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	lambdasdk "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/mashiike/lambdastub"
	"github.com/stretchr/testify/require"
)

func TestInvokeEndpointSuccess(t *testing.T) {
	type TestResponse struct {
		Name    string
		Success bool
	}
	expectedPayload := map[string]interface{}{
		"Name":    "Sample",
		"Success": true,
	}
	mux, err := lambdastub.New(
		lambdastub.WithInvokeEndpoint(map[string]interface{}{
			"HelloWorldFunction": func(payload json.RawMessage) (*TestResponse, error) {
				var p interface{}
				err := json.Unmarshal(payload, &p)
				t.Logf("payload: %#v", payload)
				require.NoError(t, err)
				require.EqualValues(t, expectedPayload, p)
				return &TestResponse{
					Name:    "name",
					Success: true,
				}, nil
			},
		}),
	)
	require.NoError(t, err)
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewLambdaClient(server)
	payload, err := json.Marshal(expectedPayload)
	require.NoError(t, err)
	output, err := client.Invoke(context.Background(), &lambdasdk.InvokeInput{
		FunctionName: aws.String("HelloWorldFunction"),
		Payload:      payload,
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"Name":"name", "Success": true}`, string(output.Payload))
}

func TestInvokeEndpointError(t *testing.T) {
	type TestResponse struct {
		Name    string
		Success bool
	}
	expectedPayload := map[string]interface{}{
		"Name":    "Sample",
		"Success": true,
	}
	mux, err := lambdastub.New(
		lambdastub.WithInvokeEndpoint(map[string]interface{}{
			"HelloWorldFunction": func(payload json.RawMessage) (*TestResponse, error) {
				var p interface{}
				err := json.Unmarshal(payload, &p)
				t.Logf("payload: %#v", payload)
				require.NoError(t, err)
				require.EqualValues(t, expectedPayload, p)
				return nil, errors.New("test error")
			},
		}),
	)
	require.NoError(t, err)
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewLambdaClient(server)
	payload, err := json.Marshal(expectedPayload)
	require.NoError(t, err)
	output, err := client.Invoke(context.Background(), &lambdasdk.InvokeInput{
		FunctionName: aws.String("HelloWorldFunction"),
		Payload:      payload,
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"errorMessage":"test error", "errorType":"errorString"}`, string(output.Payload))
	require.EqualValues(t, "errorString", *output.FunctionError)
}

func NewLambdaClient(server *httptest.Server) *lambdasdk.Client {
	return lambdasdk.New(
		lambdasdk.Options{
			Credentials: credentials.NewStaticCredentialsProvider("ACCESS_KEY_ID", "SECRET_KEY", "TOKEN"),
			EndpointResolver: lambdasdk.EndpointResolverFunc(func(region string, options lambdasdk.EndpointResolverOptions) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               server.URL,
					PartitionID:       "aws",
					SigningRegion:     "us-west-1",
					HostnameImmutable: true,
				}, nil
			}),
		},
	)
}
