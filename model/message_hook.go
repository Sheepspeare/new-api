package model

import (
	"errors"
	"gorm.io/gorm"
	"time"
)

type MessageHook struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex:idx_message_hook_name"`
	Description string `json:"description" gorm:"type:text"`
	Type        int    `json:"type" gorm:"type:int;not null;default:1;index:idx_message_hook_type"` // 1=Lua, 2=HTTP
	Content     string `json:"content" gorm:"type:text"`                                             // Lua script or HTTP URL
	Enabled     bool   `json:"enabled" gorm:"type:boolean;default:false;index:idx_message_hook_enabled"`
	Priority    int    `json:"priority" gorm:"type:int;default:0;index:idx_message_hook_priority"` // Lower number = higher priority
	Timeout     int    `json:"timeout" gorm:"type:int;default:5000"`                                // Timeout in milliseconds

	// Filter conditions (JSON stored as TEXT for cross-DB compatibility)
	FilterUsers  string `json:"filter_users" gorm:"type:text"`  // JSON array of user IDs
	FilterModels string `json:"filter_models" gorm:"type:text"` // JSON array of model names
	FilterTokens string `json:"filter_tokens" gorm:"type:text"` // JSON array of token IDs

	// Statistics
	CallCount    int64   `json:"call_count" gorm:"type:bigint;default:0"`
	SuccessCount int64   `json:"success_count" gorm:"type:bigint;default:0"`
	ErrorCount   int64   `json:"error_count" gorm:"type:bigint;default:0"`
	AvgDuration  float64 `json:"avg_duration" gorm:"type:double precision;default:0"` // Average execution time in ms

	CreatedTime int64 `json:"created_time" gorm:"bigint;index:idx_message_hook_created_time"`
	UpdatedTime int64 `json:"updated_time" gorm:"bigint"`
}

func (MessageHook) TableName() string {
	return "message_hooks"
}

// GetMessageHook retrieves a single message hook by ID
func GetMessageHook(id int) (*MessageHook, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	hook := &MessageHook{}
	err := DB.Where("id = ?", id).First(hook).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("message hook not found")
		}
		return nil, err
	}
	return hook, nil
}

// GetAllMessageHooks retrieves all message hooks with pagination and optional filtering
func GetAllMessageHooks(page, pageSize int, enabled *bool) ([]*MessageHook, int64, error) {
	var hooks []*MessageHook
	var total int64

	db := DB.Model(&MessageHook{})

	// Apply enabled filter if provided
	if enabled != nil {
		db = db.Where("enabled = ?", *enabled)
	}

	// Get total count
	err := db.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		db = db.Offset(offset).Limit(pageSize)
	}

	// Order by priority (ascending) and created_time (descending)
	err = db.Order("priority ASC, created_time DESC").Find(&hooks).Error
	return hooks, total, err
}

// GetEnabledMessageHooks retrieves all enabled message hooks sorted by priority
func GetEnabledMessageHooks() ([]*MessageHook, error) {
	var hooks []*MessageHook
	err := DB.Where("enabled = ?", true).
		Order("priority ASC, created_time DESC").
		Find(&hooks).Error
	return hooks, err
}

// CreateMessageHook creates a new message hook
func CreateMessageHook(hook *MessageHook) error {
	if hook == nil {
		return errors.New("hook is nil")
	}

	// Set timestamps
	now := time.Now().Unix()
	hook.CreatedTime = now
	hook.UpdatedTime = now

	// Initialize statistics
	hook.CallCount = 0
	hook.SuccessCount = 0
	hook.ErrorCount = 0
	hook.AvgDuration = 0

	return DB.Create(hook).Error
}

// UpdateMessageHook updates an existing message hook
func UpdateMessageHook(hook *MessageHook) error {
	if hook == nil {
		return errors.New("hook is nil")
	}
	if hook.Id == 0 {
		return errors.New("hook id is required")
	}

	// Update timestamp
	hook.UpdatedTime = time.Now().Unix()

	// Update all fields except statistics and timestamps
	return DB.Model(&MessageHook{}).
		Where("id = ?", hook.Id).
		Updates(map[string]interface{}{
			"name":          hook.Name,
			"description":   hook.Description,
			"type":          hook.Type,
			"content":       hook.Content,
			"enabled":       hook.Enabled,
			"priority":      hook.Priority,
			"timeout":       hook.Timeout,
			"filter_users":  hook.FilterUsers,
			"filter_models": hook.FilterModels,
			"filter_tokens": hook.FilterTokens,
			"updated_time":  hook.UpdatedTime,
		}).Error
}

// DeleteMessageHook deletes a message hook by ID
func DeleteMessageHook(id int) error {
	if id == 0 {
		return errors.New("id is required")
	}
	return DB.Where("id = ?", id).Delete(&MessageHook{}).Error
}

// UpdateMessageHookStats updates the statistics for a message hook
func UpdateMessageHookStats(hookId int, success bool, duration time.Duration) error {
	if hookId == 0 {
		return errors.New("hook id is required")
	}

	durationMs := float64(duration.Milliseconds())

	return DB.Transaction(func(tx *gorm.DB) error {
		// Get current hook
		var hook MessageHook
		if err := tx.Where("id = ?", hookId).First(&hook).Error; err != nil {
			return err
		}

		// Update statistics
		hook.CallCount++
		if success {
			hook.SuccessCount++
		} else {
			hook.ErrorCount++
		}

		// Update average duration using exponential moving average
		// Formula: new_avg = (old_avg * old_count + new_value) / new_count
		if hook.CallCount == 1 {
			hook.AvgDuration = durationMs
		} else {
			hook.AvgDuration = (hook.AvgDuration*float64(hook.CallCount-1) + durationMs) / float64(hook.CallCount)
		}

		// Save updates
		return tx.Model(&MessageHook{}).
			Where("id = ?", hookId).
			Updates(map[string]interface{}{
				"call_count":    hook.CallCount,
				"success_count": hook.SuccessCount,
				"error_count":   hook.ErrorCount,
				"avg_duration":  hook.AvgDuration,
			}).Error
	})
}


