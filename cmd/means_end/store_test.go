package main

import (
	"math/rand/v2"
	"sort"
	"testing"
)

type seekTest struct {
	st       store
	expected map[int32]int
}

var testCases = []seekTest{
	// Basic case - exact match on odd length
	{
		st: store{
			data: []*record{
				{date: 1, price: 0},
				{date: 2, price: 0},
				{date: 3, price: 0},
				{date: 4, price: 0},
				{date: 5, price: 0},
			},
		},
		expected: map[int32]int{
			1: 0,
			2: 1,
			3: 2,
			4: 3,
			5: 4,
			6: -1,
		},
	},
	// Basic case - exact match on even length
	{
		st: store{
			data: []*record{
				{date: 1, price: 0},
				{date: 2, price: 0},
				{date: 3, price: 0},
				{date: 4, price: 0},
				{date: 5, price: 0},
				{date: 6, price: 0},
			},
		},
		expected: map[int32]int{
			1: 0,
			2: 1,
			3: 2,
			4: 3,
			5: 4,
			6: 5,
			7: -1,
		},
	},
	// Non-exact match on even length
	{
		st: store{
			data: []*record{
				{date: 1, price: 0},
				{date: 2, price: 0},
				{date: 3, price: 0},
				{date: 5, price: 0},
				{date: 6, price: 0},
				{date: 7, price: 0},
			},
		},
		expected: map[int32]int{
			1: 0,
			2: 1,
			3: 2,
			4: 3, // 4 is not present, element 5(index 3) is the closest match
			5: 3,
			6: 4,
			7: 5,
			8: -1,
		},
	},
	// Non-exact match on odd length
	{
		st: store{
			data: []*record{
				{date: 1, price: 0},
				{date: 2, price: 0},
				{date: 3, price: 0},
				{date: 5, price: 0},
				{date: 6, price: 0},
			},
		},
		expected: map[int32]int{
			1: 0,
			2: 1,
			3: 2,
			4: 3, // 4 is not present, element 5(index 3) is the closest match
			5: 3,
			6: 4,
			7: -1,
		},
	},
	// Large store - exact match
	{
		st: genLargeStore(50000),
		expected: map[int32]int{
			25000: 25000,
			50001: -1,
		},
	},
}

func TestSeekStart(t *testing.T) {
	for i, tc := range testCases {
		t.Logf("Testing store #%d", i)
		for targetDate, expStart := range tc.expected {
			t.Logf("Seeking %d, expecting %d", targetDate, expStart)

			t.Logf("Seeking seekStart")
			actualStart := tc.st.seekStart(targetDate)
			t.Logf("Seeking seekStartSeq")
			actualStartSeq := tc.st.seekStartSeq(targetDate)

			if actualStart != expStart || actualStartSeq != expStart {
				t.Errorf("FAIL expected: %d, actual: %d, actualSeq: %d", expStart, actualStart, actualStartSeq)
			}
		}
	}
}

func TestStoreSortedProperty(t *testing.T) {
	st := newStore()
	count := 100

	t.Logf("Populating the store")
	for i := range count {
		rec := genRandomRecord()
		t.Logf("Generated record %d: %d", i, rec.date)
		st.insert(rec)
	}
	/*
		sortFunc := func(a record, b record) int {
			if a.date > b.date {
				return 1
			} else if a.date < b.date {
				return -1
			} else {
				return 0
			}
		}
	*/

	t.Logf("Creating the expected data")
	expected := make([]*record, len(st.data))
	for i := range len(st.data) {
		expected[i] = st.data[i]
	}

	t.Logf("Sorting the expected data")
	sort.Slice(expected, func(i, j int) bool {
		return expected[i].date < expected[j].date
	})

	st.sort()

	t.Logf("Comparing the actual & expected data")
	for i := range len(st.data) {
		if st.data[i] != expected[i] {
			t.Errorf("FAIL expected[%d]: %d; actual[%d]: %d", i, expected[i], i, st.data[i])
		}
	}

}

func BenchmarkSeekStart(b *testing.B) {
	st := genLargeStore(600000)
	for b.Loop() {
		st.seekStart(300000)
	}
}

func BenchmarkSeekStartSeq(b *testing.B) {
	st := genLargeStore(600000)
	for b.Loop() {
		st.seekStartSeq(300000)
	}
}

func genLargeStore(size int) store {
	res := make([]*record, size)

	for i := range size {
		res[i] = &record{date: int32(i), price: 0}
	}

	return store{
		data: res,
	}
}

func genRandomRecord() *record {
	price, date := rand.Int32(), rand.Int32()
	if price < 0 {
		price *= -1
	}

	if date < 0 {
		date *= -1
	}
	return &record{date: date, price: price}
}
