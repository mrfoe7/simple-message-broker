package main

import "errors"

var (
	errTypeCast = errors.New("type cast")

	errEmptyQueue = errors.New("empty queue")
)
