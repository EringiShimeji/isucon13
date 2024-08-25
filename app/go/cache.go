package main

import (
	"sync"
)

type Cache struct {
	userImageHash   sync.Map // username -> imageHash
	userTheme       sync.Map // user_id -> theme
	userModel       sync.Map // user_id -> UserModel
	livestreamTags  sync.Map // livestream_id -> tags
	livestreamModel sync.Map // livestream_id -> LivestreamModel
}

var cache Cache

func (c *Cache) getLivestreamModel(livestreamID int64) (LivestreamModel, bool) {
	m, ok := cache.livestreamModel.Load(livestreamID)
	if !ok {
		return LivestreamModel{}, ok
	}
	return m.(LivestreamModel), ok
}

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

	var livestreamModels []LivestreamModel
	if err := dbConn.Select(&livestreamModels, "SELECT * FROM livestreams"); err != nil {
		return err
	}
	for _, model := range livestreamModels {
		cache.livestreamModel.Store(model.ID, model)
	}

	return nil
}
