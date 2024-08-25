package main

import (
	"sync"
)

type Cache struct {
	userImageHash  sync.Map // username -> imageHash
	userTheme      sync.Map // user_id -> theme
	userModel      sync.Map // user_id -> UserModel
	livestreamTags sync.Map // livestream_id -> tags
}

var cache Cache

func (c *Cache) getUserModel(userID int64) (UserModel, bool) {
	userModel, ok := cache.userModel.Load(userID)
	if userModel == nil {
		return UserModel{}, ok
	}
	return userModel.(UserModel), ok
}

func InitCache() error {
	var userModels []UserModel
	if err := dbConn.Select(&userModels, "SELECT * FROM users"); err != nil {
		return err
	}
	for _, model := range userModels {
		cache.userModel.Store(model.ID, model)
	}
	return nil
}
