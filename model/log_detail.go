package model

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

type LogDetail struct {
	Id                int    `json:"id" gorm:"primaryKey;autoIncrement"`
	RequestId         string `json:"request_id" gorm:"type:varchar(64);uniqueIndex:idx_log_details_request_id;not null"`
	RequestBody       string `json:"request_body" gorm:"type:mediumtext"`
	ResponseBody      string `json:"response_body" gorm:"type:mediumtext"`
	ExtractedContent  string `json:"extracted_content" gorm:"type:mediumtext"` // 提取的文本内容（流式响应组装后的文本）
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index"`
}

func (LogDetail) TableName() string {
	return "log_details"
}

// truncateToMaxSize 截断字符串到最大字节数
func truncateToMaxSize(s string, maxSize int) string {
	if maxSize <= 0 || len(s) <= maxSize {
		return s
	}
	// 确保不在 UTF-8 多字节字符中间截断
	truncated := s[:maxSize]
	// 从末尾往前找到完整的 UTF-8 字符边界
	for i := len(truncated) - 1; i >= len(truncated)-4 && i >= 0; i-- {
		if truncated[i]&0xC0 != 0x80 { // 找到 UTF-8 字符的起始字节
			// 检查这个字符是否完整
			r := truncated[i]
			var charLen int
			switch {
			case r&0x80 == 0:
				charLen = 1
			case r&0xE0 == 0xC0:
				charLen = 2
			case r&0xF0 == 0xE0:
				charLen = 3
			case r&0xF8 == 0xF0:
				charLen = 4
			default:
				charLen = 1
			}
			if i+charLen > len(truncated) {
				truncated = truncated[:i]
			}
			break
		}
	}
	return truncated + "\n...[内容已截断]"
}

// RecordLogDetail 异步记录请求/响应详情
func RecordLogDetail(requestId string, requestBody string, responseBody string, extractedContent string) {
	if !common.LogDetailEnabled || requestId == "" {
		return
	}

	maxSize := common.LogDetailMaxSize

	requestBody = truncateToMaxSize(requestBody, maxSize)
	responseBody = truncateToMaxSize(responseBody, maxSize)
	extractedContent = truncateToMaxSize(extractedContent, maxSize)

	gopool.Go(func() {
		detail := &LogDetail{
			RequestId:        requestId,
			RequestBody:      requestBody,
			ResponseBody:     responseBody,
			ExtractedContent: extractedContent,
			CreatedAt:        common.GetTimestamp(),
		}
		err := LOG_DB.Create(detail).Error
		if err != nil {
			common.SysLog("failed to record log detail: " + err.Error())
		}
	})
}

// GetLogDetailByRequestId 根据 request_id 查询日志详情
func GetLogDetailByRequestId(requestId string) (*LogDetail, error) {
	var detail LogDetail
	err := LOG_DB.Where("request_id = ?", requestId).First(&detail).Error
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

// DeleteOldLogDetail 删除指定时间之前的日志详情
func DeleteOldLogDetail(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0
	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}
		
		var result *gorm.DB
		if common.UsingPostgreSQL {
			// PostgreSQL: 使用子查询（PostgreSQL 不支持 DELETE ... LIMIT）
			result = LOG_DB.Where("id IN (?)",
				LOG_DB.Model(&LogDetail{}).
					Select("id").
					Where("created_at < ?", targetTimestamp).
					Limit(limit),
			).Delete(&LogDetail{})
		} else {
			// MySQL/SQLite: 直接使用 LIMIT
			result = LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&LogDetail{})
		}
		
		if nil != result.Error {
			return total, result.Error
		}
		total += result.RowsAffected
		if result.RowsAffected < int64(limit) {
			break
		}
		
		// 添加短暂延迟，避免持续占用数据库资源
		if total > 0 && result.RowsAffected == int64(limit) {
			select {
			case <-ctx.Done():
				return total, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				// 继续下一批
			}
		}
	}
	return total, nil
}
