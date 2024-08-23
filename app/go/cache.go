package main

import "sync"

type Cache struct {
	userIdHash sync.Map
}

var cache Cache

func InitCache() {
}
