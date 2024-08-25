package main

import "sync"

type Cache struct {
	userImageHash sync.Map // username -> imageHash
	userTheme     sync.Map // user_id -> theme
	userModel     sync.Map // user_id -> UserModel
}

var cache Cache

func InitCache() {
}
