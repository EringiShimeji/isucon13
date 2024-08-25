package main

import "sync"

type Cache struct {
	usernameHash sync.Map
	userTheme    sync.Map // user_id -> theme
}

var cache Cache

func InitCache() {
}
