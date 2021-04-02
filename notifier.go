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
	// listners map[string]*Conn
	listners map[string]interface{}

	l sync.RWMutex
}

func NewNotifier() *Notifier {
	return &Notifier{
		listners: make(map[string]interface{}),
	}
}

func (n *Notifier) Notify(key string) {
	n.l.RLock()
	defer n.l.RUnlock()

	if connV, exists := n.listners[key]; exists {
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

	n.l.Lock()
	n.listners[key] = &Conn{
		ctx, ch,
	}
	n.l.Unlock()

	return ch
}
