package main

import "sync"

type Cache struct {
	usernameHash sync.Map
}

var cache Cache

func InitCache() {
}
