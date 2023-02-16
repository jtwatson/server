// server starts an HTTP Server to run the application
package server

import (
	"context"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Parallel()

	type args struct {
		addr string
	}
	tests := []struct {
		name string
		args args
		want *Server
	}{
		{
			name: "New",
			args: args{addr: "myhost:2000"},
			want: &Server{
				srv: &http.Server{
					Addr:              "myhost:2000",
					ReadHeaderTimeout: 60 * time.Second,
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := New(tt.args.addr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServer_Start(t *testing.T) {
	type args struct {
		responseDelay time.Duration
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Request completes",
			args: args{responseDelay: time.Millisecond},
		},
		{
			name:    "Request incomplete",
			args:    args{responseDelay: time.Second * 10},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var wg sync.WaitGroup
			var handlerCalled atomic.Bool
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wg.Done()
				if handlerCalled.Load() {
					time.Sleep(tt.args.responseDelay)
				}
				handlerCalled.Store(true)
			})

			s := &Server{
				srv: &http.Server{
					Addr:              "localhost:23461",
					ReadHeaderTimeout: 60 * time.Second,
				},
			}

			done := make(chan bool)
			go func() {
				defer close(done)

				if err := s.Start(handler); (err != nil) != tt.wantErr {
					t.Errorf("Server.Start() error = %v, wantErr %v", err, tt.wantErr)
				}
				done <- true
			}()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:23461", http.NoBody)
			if err != nil {
				t.Fatalf("http.NewRequest() error = %v", err)
			}

			var res *http.Response
			for i := 0; i <= 5; i++ {
				wg.Add(1)
				var err error
				res, err = http.DefaultClient.Do(req)
				if err == nil {
					break
				}
				if i == 5 {
					t.Fatalf("http.Do() error = %v", err)
				}
				time.Sleep(time.Millisecond * time.Duration(i*20))
				wg.Done()
			}

			if res.StatusCode != http.StatusOK {
				t.Fatalf("res.StatusCode = %v, want %v", res.StatusCode, http.StatusOK)
			}

			if called := handlerCalled.Load(); !called {
				t.Fatalf("handlerCalled = %v", called)
			}

			// second request that will delay via tt.args.responseDelay
			wg.Add(1)
			go func() {
				_, _ = http.DefaultClient.Do(req)
			}()

			// Wait for handler to signal it has been invoked
			wg.Wait()

			// send shutdown
			_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)

			select {
			case <-done:
			case <-time.After(time.Second * 10):
				t.Fatalf("srv.Shutdown() did not return within 10 seconds")
			}
		})
	}
}
