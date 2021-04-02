package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

const (
	defaultPort   = 80
	shutdownDelay = 1 * time.Second

	portFlag  = "p"
	routerKey = "key"

	valueArg   = "v"
	timeoutArg = "timeout"

	warningTmpl = "warning: %s\n"
	errorTmpl   = "error: %s\n"
)

var globalQueue *GlobalQueue

type GlobalQueue struct {
	notifier *Notifier

	queues map[string]*Queue

	l sync.Mutex
}

func (gq *GlobalQueue) Shift(key string) (string, error) {
	gq.l.Lock()
	defer gq.l.Unlock()

	q, exists := gq.queues[key]
	if !exists {
		return "", errEmptyQueue
	}

	valV, err := q.Shift()
	if err != nil {
		return "", err
	}

	val, ok := valV.(string)
	if !ok {
		log.Printf(errorTmpl, "cast queue to *Conn type")
		return "", errTypeCast
	}

	return val, nil
}

func (gq *GlobalQueue) Push(key, val string) {
	var (
		q  *Queue
		ok bool
	)

	gq.l.Lock()
	q, ok = gq.queues[key]
	if !ok {
		q = &Queue{}
	}

	q.Push(val)

	if !ok {
		gq.queues[key] = q
	}
	gq.l.Unlock()

	gq.notifier.Notify(key)
}

func NewGlobalQueue() *GlobalQueue {
	return &GlobalQueue{
		queues:   make(map[string]*Queue),
		notifier: NewNotifier(),
	}
}

func GetResponse(gq *GlobalQueue, key string, timeout int64) (status int, body []byte) {
	val, err := gq.Shift(key)

	switch {
	case err != nil && err != errEmptyQueue:
		status = http.StatusInternalServerError
		log.Printf(warningTmpl, err)

	case err == errEmptyQueue && timeout <= 0:
		status = http.StatusNotFound
		body = []byte("{ошибка, 404 not found}")

	case err == errEmptyQueue && timeout > 0:
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)

		defer func() {
			cancel()
		}()

		ch := gq.notifier.Subscribe(ctx, key)
	loop:
		for {
			select {
			case <-ctx.Done():
				status = http.StatusNotFound
				body = []byte("{ошибка, 404 not found}")

				break loop
			case <-ch:
				timeout = 0
				return GetResponse(gq, key, timeout)
			}
		}

	default:
		status = http.StatusOK
		body = []byte(val)
	}

	return status, body
}

func getRequestKey(req *http.Request) string {
	vars := mux.Vars(req)
	return vars[routerKey]
}

func putHandler(w http.ResponseWriter, req *http.Request) {
	key := getRequestKey(req)

	query := req.URL.Query()
	val := query.Get(valueArg)

	respStatus := http.StatusOK
	if val != "" {
		globalQueue.Push(key, val)
	} else {
		log.Printf(warningTmpl, "empty val into query")
		respStatus = http.StatusBadRequest
	}

	w.WriteHeader(respStatus)
}

func getHandler(w http.ResponseWriter, req *http.Request) {
	key := getRequestKey(req)

	query := req.URL.Query()
	timeoutStr := query.Get(timeoutArg)

	var (
		timeout int64
		err     error
	)

	if timeoutStr != "" {
		timeout, err = strconv.ParseInt(timeoutStr, 10, 0)
		if err != nil {
			log.Printf(warningTmpl, err)
		}
	}

	httpStatus, body := GetResponse(globalQueue, key, timeout)
	w.WriteHeader(httpStatus)
	if body != nil {
		w.Write([]byte(body))
	}
}

func main() {
	flValue := flag.Int(portFlag, defaultPort, "web server port")
	flag.Parse()

	port := *flValue

	if port == defaultPort {
		log.Printf(warningTmpl, "using default port")
	}

	globalQueue = NewGlobalQueue()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	router := mux.NewRouter()

	router.HandleFunc(fmt.Sprintf("/{%s}", routerKey), getHandler).Methods(http.MethodGet)
	router.HandleFunc(fmt.Sprintf("/{%s}", routerKey), putHandler).Methods(http.MethodPut)

	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	log.Printf("server started by :%v", port)

	<-done

	//shutdown delay
	log.Printf("shutdown delay: %v\n", shutdownDelay)

	time.Sleep(shutdownDelay)

	log.Println("server stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer func() {
		cancel()
	}()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown failed %s\n", err)
	}

	log.Println("server is shutdown")
}
