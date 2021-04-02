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

	l sync.RWMutex
}

func NewNotifier() *Notifier {
	return &Notifier{
		listners: make(map[string]*Queue),
	}
}

func (n *Notifier) Notify(key string) {
	n.l.RLock()
	defer n.l.RUnlock()

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

	q, exists = n.listners[key]
	if !exists {
		q = &Queue{}
	}

	q.Push(&Conn{
		ctx, ch,
	})

	if !exists {
		n.listners[key] = q
	}

	n.l.Unlock()

	return ch
}
