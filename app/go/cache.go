package main

import "sync"

type Cache struct {
	usernameHash   sync.Map
	userTheme      sync.Map // user_id -> theme
	livestreamTags sync.Map // livestream_id -> tags
}

var cache Cache

func InitCache() {
}
