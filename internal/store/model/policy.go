package model

import (
	"time"
)

type Policy struct {
	ID            string                 `gorm:"primaryKey;type:varchar(63)"`
	DisplayName   string                 `gorm:"column:display_name;not null"`
	Description   string                 `gorm:"column:description"`
	PolicyType    string                 `gorm:"column:policy_type;not null"`
	LabelSelector map[string]string      `gorm:"column:label_selector;serializer:json"`
	Priority      int32                  `gorm:"column:priority;not null"`
	Enabled       bool                   `gorm:"column:enabled;not null"`
	CreateTime    time.Time              `gorm:"column:create_time;autoCreateTime"`
	UpdateTime    time.Time              `gorm:"column:update_time;autoUpdateTime"`
}

type PolicyList []Policy
