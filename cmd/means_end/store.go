package main

import (
	"fmt"
	"slices"
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
	data []record
}

func newStore() *store {
	return &store{
		data: []record{},
	}
}

func (s *store) insert(r record) {
	if len(s.data) == 0 {
		s.data = append(s.data, r)
		return
	}

	for i := range len(s.data) {
		if i == len(s.data)-1 {
			if r.date <= s.data[i].date {
				s.data = slices.Insert(s.data, i, r)
			} else {
				s.data = append(s.data, r)
			}
		} else {
			if r.date <= s.data[i].date {
				s.data = slices.Insert(s.data, i, r)
			}
		}
	}
}

func (s store) mean(startDate, endDate int32) int32 {
	if endDate < startDate {
		return 0
	}

	startPos := s.seek(startDate)
	if startPos == -1 {
		return 0
	}

	var total, count int32
	for i := startPos; i < len(s.data) && s.data[i].date <= endDate; i++ {
		total += s.data[i].price
		count++
	}

	if count == 0 {
		return 0
	}

	return total / count
}

func (s store) seek(targetDate int32) int {
	for i := range len(s.data) {
		if s.data[i].date >= targetDate {
			return i
		}
	}

	return -1
}

func (s store) String() string {
	res := ""
	for _, record := range s.data {
		res += fmt.Sprintf("%d - %d\n", record.date, record.price)
	}

	return res
}
