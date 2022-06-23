package lambdastub

import (
	"net/http"

	"github.com/gorilla/mux"
)

type Stub struct {
	r         *mux.Router
	endpoints map[string]Endpoint
}

type StubOptions struct {
	Endpoints map[string]Endpoint
}

type Endpoint interface {
	Register(r *mux.Router) error
	http.Handler
}

func New(optFns ...func(*StubOptions) error) (*Stub, error) {
	opts := StubOptions{
		Endpoints: make(map[string]Endpoint),
	}
	for _, optFn := range optFns {
		if err := optFn(&opts); err != nil {
			return nil, err
		}
	}
	stub := &Stub{
		r:         mux.NewRouter(),
		endpoints: opts.Endpoints,
	}
	for _, endpoint := range stub.endpoints {
		if err := endpoint.Register(stub.r); err != nil {
			return nil, err
		}
	}
	stub.r.Handle("*", http.NotFoundHandler())
	return stub, nil
}

func (stub *Stub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	stub.r.ServeHTTP(w, r)
}
