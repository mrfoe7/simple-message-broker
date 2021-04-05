package main

import (
	"context"
	"log"
	"sync"
)

type Conn struct {
	ctx context.Context
	ch  chan struct{}
}

type Notifier struct {
	listners map[string]*Queue

	l sync.Mutex
}

func NewNotifier() *Notifier {
	return &Notifier{
		listners: make(map[string]*Queue),
	}
}

func (n *Notifier) Notify(key string) {
	n.l.Lock()
	defer n.l.Unlock()

	if q, exists := n.listners[key]; exists {
		connV, err := q.Shift()
		if err != nil {
			log.Printf(errorTmpl, err)
			return
		}
		conn, ok := connV.(*Conn)
		if !ok {
			log.Printf(errorTmpl, "cast queue to *Conn type")
			return
		}

		select {
		case <-conn.ctx.Done():
		default:
			conn.ch <- struct{}{}
			close(conn.ch)
		}
	}
}

func (n *Notifier) Subscribe(ctx context.Context, key string) <-chan struct{} {
	ch := make(chan struct{})

	var (
		q      *Queue
		exists bool
	)

	n.l.Lock()
	defer n.l.Unlock()

	q, exists = n.listners[key]
	if !exists {
		q = &Queue{}
		n.listners[key] = q
	}

	q.Push(&Conn{
		ctx, ch,
	})

	return ch
}
