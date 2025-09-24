package main

import (
	"fmt"
	"slices"
	"sort"
)

type record struct {
	date  int32
	price int32
}

func newRecord(date, price int32) *record {
	return &record{
		date:  date,
		price: price,
	}
}

type store struct {
	data []*record
}

func newStore() *store {
	return &store{
		data: []*record{},
	}
}

func (s *store) insertOld(r *record) {
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

func (s *store) insert(r *record) {
	s.data = append(s.data, r)
}

func (s *store) sort() {
	sort.Slice(s.data, func(i, j int) bool {
		return s.data[i].date < s.data[j].date
	})
}

func (s store) mean(startDate, endDate int32) int32 {
	if endDate < startDate {
		return 0
	}

	s.sort()

	startPos := s.seekStart(startDate)
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

// Either find an exact match or find the point
// where the start is after the previous date but
// before the current one
func (s store) seekStartOld(targetDate int32) int {
	if len(s.data) == 0 {
		return -1
	}

	i := len(s.data) / 2
	for i >= 0 && i < len(s.data) {
		curDate := s.data[i].date
		if curDate == targetDate {
			return i
		}

		if i > 0 && i < len(s.data)-1 {
			if targetDate > s.data[i-1].date && targetDate < curDate {
				return i
			} else if targetDate < curDate {
				i = i / 2
			} else {
				i = i + (len(s.data)-i)/2
			}
		} else if i == 0 && targetDate < curDate {
			return i
		} else if i == len(s.data)-1 && targetDate < curDate {
			return i
		} else {
			return -1
		}
	}

	return -1
}

func (s *store) seekStart(target int32) int {
	if len(s.data) == 0 {
		return -1
	}

	start, end := 0, len(s.data)-1

	var i int
	for start <= end {
		i = start + (end-start)/2
		curDate := s.data[i].date

		if curDate == target {
			return i
		}

		// An exact match cannot be found, we're at the pivot and the target is missing
		if i > 0 && target < curDate && target > s.data[i-1].date {
			return i
		}

		if target > curDate {
			start = i + 1
		} else {
			end = i - 1
		}
	}

	if target < s.data[i].date {
		return i
	}

	return -1
}

func (s *store) seekStartSeq(targetDate int32) int {
	for i, curRec := range s.data {
		if targetDate == curRec.date || targetDate < curRec.date {
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
