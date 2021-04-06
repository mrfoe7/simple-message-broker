package main

type Queue struct {
	data []interface{}
}

func (q *Queue) Shift() (interface{}, error) {
	if len(q.data) == 0 {
		return "", errEmptyQueue
	}

	val := q.data[0]
	q.data = q.data[1:]

	return val, nil
}

func (q *Queue) Push(val interface{}) {
	q.data = append(q.data, val)
}

func (q *Queue) Get() (interface{}, error) {
	if len(q.data) == 0 {
		return "", errEmptyQueue
	}

	return q.data[0], nil
}
