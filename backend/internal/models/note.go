package models

import "gorm.io/gorm"

type Note struct {
	gorm.Model
	Title   string `gorm:"size:255;not null"`
	Content string `gorm:"type:text"`
	Tags    string `gorm:"type:text"`
}
