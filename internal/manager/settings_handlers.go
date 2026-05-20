package manager

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cronicle/cronicle-dealer/internal/models"
	"github.com/cronicle/cronicle-dealer/internal/storage"
)

const systemConfigID = "default"

func (s *APIServer) getSettings(c *gin.Context) {
	settings := s.currentEditableSettings()
	dbOverride := false

	var sysCfg models.SystemConfig
	if err := storage.DB.Where("id = ?", systemConfigID).First(&sysCfg).Error; err == nil && sysCfg.Content != "" {
		var override map[string]interface{}
		if err := json.Unmarshal([]byte(sysCfg.Content), &override); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库配置格式错误"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"config":      override,
			"db_override": true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config":      settings,
		"db_override": dbOverride,
	})
}

func (s *APIServer) updateSettings(c *gin.Context) {
	var req struct {
		Config map[string]interface{} `json:"config" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	content, err := json.Marshal(req.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "配置格式错误: " + err.Error()})
		return
	}

	updatedBy, _ := c.Get("username")
	sysCfg := models.SystemConfig{
		ID:        systemConfigID,
		Content:   string(content),
		UpdatedBy: stringValue(updatedBy),
	}

	if err := storage.DB.Save(&sysCfg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置已保存"})
}

func (s *APIServer) currentEditableSettings() map[string]interface{} {
	return map[string]interface{}{
		"manager": map[string]interface{}{
			"heartbeat": map[string]interface{}{
				"timeout":         s.cfg.Manager.Heartbeat.Timeout,
				"check_interval":  s.cfg.Manager.Heartbeat.CheckInterval,
				"pending_timeout": s.cfg.Manager.Heartbeat.PendingTimeout,
			},
			"dispatch_retry": map[string]interface{}{
				"max_retries":    s.cfg.Manager.DispatchRetry.MaxRetries,
				"base_delay_sec": s.cfg.Manager.DispatchRetry.BaseDelaySec,
				"max_delay_sec":  s.cfg.Manager.DispatchRetry.MaxDelaySec,
			},
			"history": map[string]interface{}{
				"event_retention_days":  s.cfg.Manager.History.EventRetentionDays,
				"metric_retention_days": s.cfg.Manager.History.MetricRetentionDays,
			},
		},
		"logging": map[string]interface{}{
			"level":              s.cfg.Logging.Level,
			"format":             s.cfg.Logging.Format,
			"output":             s.cfg.Logging.Output,
			"log_dir":            s.cfg.Logging.LogDir,
			"log_retention_days": s.cfg.Logging.LogRetentionDays,
			"max_log_size_mb":    s.cfg.Logging.MaxLogSizeMB,
		},
	}
}

func stringValue(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
