package main

import (
	"math/rand"
	"slices"
	"sync"
)

type record struct {
	date  int32
	price int32
}

func newRecord(date, price int32) record {
	return record{
		date:  date,
		price: price,
	}
}

type store struct {
	data map[int][]record
	lock *sync.RWMutex
}

func newStore() *store {
	return &store{
		data: make(map[int][]record),
		lock: &sync.RWMutex{},
	}
}

func (s store) newSession() int {
	id := rand.Int()

	s.lock.Lock()
	defer s.lock.Unlock()
	_, present := s.data[id]
	for present {
		id = rand.Int()
		_, present = s.data[id]
	}

	s.data[id] = []record{}

	return id
}

func (s store) deleteSession(id int) {
	s.lock.Lock()
	delete(s.data, id)
	s.lock.Unlock()
}

func (s store) insert(id int, r record) {
	s.lock.Lock()
	defer s.lock.Unlock()

	curData := s.data[id]

	if len(curData) == 0 {
		s.data[id] = append(s.data[id], r)
		return
	}

	for i := range len(curData) {
		if i == len(curData)-1 {
			if r.date <= curData[i].date {
				s.data[id] = slices.Insert(s.data[id], i, r)
			} else {
				s.data[id] = append(s.data[id], r)
			}
		} else {
			if r.date <= curData[i].date {
				s.data[id] = slices.Insert(s.data[id], i, r)
			}
		}
	}
}

func (s store) mean(id int, startDate, endDate int32) int32 {
	if endDate < startDate {
		return 0
	}

	s.lock.RLock()
	defer s.lock.RUnlock()

	startPos := s.seek(id, startDate)
	if startPos == -1 {
		return 0
	}

	var total, count int32
	for i := startPos; i < len(s.data[id]) && s.data[id][i].date <= endDate; i++ {
		total += s.data[id][i].price
		count++
	}

	return total / count
}

func (s store) seek(id int, targetDate int32) int {
	for i := range len(s.data[id]) {
		if s.data[id][i].date >= targetDate {
			return i
		}
	}

	return -1
}
