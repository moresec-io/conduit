package repo

import "gorm.io/gorm"

type dao struct {
	db *gorm.DB
}
