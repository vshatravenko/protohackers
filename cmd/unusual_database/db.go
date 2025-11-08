package main

import "sync"

type database struct {
	store map[string]string
	mtx   *sync.RWMutex
}

func newDatabase() *database {
	return &database{
		store: make(map[string]string),
		mtx:   &sync.RWMutex{},
	}
}

func (db *database) insert(key, value string) {
	db.store[key] = value
}

func (db *database) retrieve(key string) string {
	val, ok := db.store[key]
	if !ok {
		return ""
	}

	return val
}
