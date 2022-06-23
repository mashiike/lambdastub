package lambdastub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/Songmu/flextime"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambda/messages"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// https://docs.aws.amazon.com/lambda/latest/dg/API_Invoke.html
type InvokeEndpoint struct {
	mu       sync.Mutex
	handlers map[string]lambda.Handler
	reg      *regexp.Regexp
}

const functionNameRegexpPattern = `(arn:(aws[a-zA-Z-]*)?:lambda:)?([a-z]{2}(-gov)?-[a-z]+-\d{1}:)?(\d{12}:)?(function:)?([a-zA-Z0-9-_\.]+)(:(\$LATEST|[a-zA-Z0-9-_]+))?`

func newInvokeEndpoint(handlerFuncByFunctionName map[string]interface{}) *InvokeEndpoint {
	endpoint := &InvokeEndpoint{
		handlers: make(map[string]lambda.Handler, len(handlerFuncByFunctionName)),
		reg:      regexp.MustCompile(functionNameRegexpPattern),
	}
	for functionName, handlerFunc := range handlerFuncByFunctionName {
		endpoint.handlers[functionName] = lambda.NewHandler(handlerFunc)
	}
	return endpoint
}

func WithInvokeEndpoint(handlerFuncByFunctionName map[string]interface{}) func(*StubOptions) error {
	return func(opts *StubOptions) error {
		opts.Endpoints["invoke"] = newInvokeEndpoint(handlerFuncByFunctionName)
		return nil
	}
}

func (e *InvokeEndpoint) Register(r *mux.Router) error {
	r.Handle("/2015-03-31/functions/{FunctionName}/invocations", e).Methods(http.MethodPost)
	return nil
}

func (e *InvokeEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	vars := mux.Vars(r)
	functionName, ok := vars["FunctionName"]
	if !ok {
		w.Header().Set("X-Amzn-ErrorType", "InvalidParameterValueException")
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "FunctionName not found")
		return
	}
	if !e.reg.MatchString(functionName) {
		w.Header().Set("X-Amzn-ErrorType", "InvalidParameterValueException")
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "FunctionName is invalid")
		return
	}
	qualifier := r.URL.Query().Get("Qualifier")

	//migration to ARN
	functionARN, err := arn.Parse(functionName)
	if err != nil {
		functionARN = arn.ARN{
			Partition: "aws",
			AccountID: "123456789012",
			Service:   "lambda",
			Region:    os.Getenv("AWS_DEFAULT_REGION"),
			Resource:  fmt.Sprintf("function:%s", functionName),
		}
		if functionARN.Region == "" {
			functionARN.Region = "us-east-1"
		}
		if qualifier != "" {
			functionARN.Resource += ":" + qualifier
		}
	} else {
		functionName = strings.TrimPrefix(functionARN.Resource, "function:")
	}

	handler, ok := e.handlers[functionName]
	if !ok {
		handler, ok = e.handlers[functionARN.String()]
		if !ok {
			w.Header().Set("X-Amzn-ErrorType", "ResourceNotFoundException")
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "Function not found: %s", functionARN.String())
			return
		}
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("X-Amzn-ErrorType", "InvalidRequestContentException")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "can not read payload: %s", err.Error())
		return
	}
	var functionVersion string
	if qualifier == "$LATEST" {
		functionVersion = qualifier
	} else if version, err := strconv.ParseUint(qualifier, 10, 64); err == nil {
		functionVersion = fmt.Sprintf("%d", version)
	} else {
		functionVersion = "1"
		if qualifier == "" {
			qualifier = "$LATEST"
		}
	}
	w.Header().Set("X-Amz-Executed-Version", functionVersion)

	var logBuf bytes.Buffer
	uuidObj, _ := uuid.NewRandom()
	reqID := uuidObj.String()
	now := flextime.Now()
	defaultWriter := log.Default().Writer()
	logWriter := io.MultiWriter(defaultWriter, &logBuf)
	defer func() {
		log.Default().SetOutput(defaultWriter)
	}()

	fmt.Fprintf(logWriter, "%s START RequestId: %s Version: %s\n", now.Format("2006/01/02 15:04:05"), reqID, qualifier)
	invocationType := r.Header.Get("X-Amz-Invocation-Type")
	if invocationType == "" {
		invocationType = "RequestResponse"
	}
	var cc lambdacontext.ClientContext
	if base64ClientContext := r.Header.Get("X-Amz-Client-Context"); base64ClientContext != "" {
		decoder := json.NewDecoder(base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64ClientContext)))
		decoder.Decode(&cc)
	}
	ctx := lambdacontext.NewContext(r.Context(), &lambdacontext.LambdaContext{
		AwsRequestID:       reqID,
		InvokedFunctionArn: functionARN.String(),
		ClientContext:      cc,
	})
	fmt.Fprintf(logWriter, "%s %s\n", now.Format("2006/01/02 15:04:05"), string(payload))
	output, err := func() ([]byte, error) {
		e.mu.Lock()
		defer e.mu.Unlock()
		lambdacontext.FunctionName = functionName
		lambdacontext.FunctionVersion = functionVersion
		lambdacontext.LogGroupName = "/aws/lambda/" + functionName
		lambdacontext.LogStreamName = now.Format("2006/01/02[") + qualifier + "]00000000000000000000000000000000"
		lambdacontext.MemoryLimitInMB = 128
		return handler.Invoke(ctx, payload)
	}()
	endTime := flextime.Now()
	if err != nil {
		var ive messages.InvokeResponse_Error
		if errors.As(err, &ive) {
			w.Header().Set("X-Amz-Function-Error", ive.Type)
		} else {
			if errorType := reflect.TypeOf(err); errorType.Kind() == reflect.Ptr {
				ive.Type = errorType.Elem().Name()
			} else {
				ive.Type = errorType.Name()
			}
			w.Header().Set("X-Amz-Function-Error", ive.Type)
			ive.Message = err.Error()
		}
		bs, _ := json.Marshal(ive)
		fmt.Fprintf(logWriter, "%s %s\n", endTime.Format("2006/01/02 15:04:05"), string(bs))
		output = bs
	} else {
		fmt.Fprintf(logWriter, "%s %s\n", endTime.Format("2006/01/02 15:04:05"), string(output))
	}
	fmt.Fprintf(logWriter, "%s END RequestId: %s\n", endTime.Format("2006/01/02 15:04:05"), reqID)
	fmt.Fprintf(logWriter, "%s REPORT RequestId: %s  Init Duration x.xx ms Duration:%02f ms     Billed Duration yyy ms Memory Size 128 MB     Max Memory Used: 128 MB\n", endTime.Format("2006/01/02 15:04:05"), reqID, float64(endTime.Sub(now).Microseconds())/1000.0)

	if r.Header.Get("X-Amz-Log-Type") == "Tail" && invocationType == "RequestResponse" {
		var buf bytes.Buffer
		enc := base64.NewEncoder(base64.RawStdEncoding, &buf)
		enc.Write(logBuf.Bytes())
		enc.Close()
		w.Header().Set("X-Amz-Log-Result", buf.String())
	}
	w.WriteHeader(http.StatusOK)
	w.Write(output)
}
