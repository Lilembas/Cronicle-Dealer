package manager

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/logger"
	"github.com/cronicle/cronicle-next/pkg/utils"
)

// listStrategies 获取所有负载均衡策略
func (s *APIServer) listStrategies(c *gin.Context) {
	var strategies []models.LoadBalanceStrategy
	if err := storage.DB.Order("created_at DESC").Find(&strategies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if strategies == nil {
		strategies = []models.LoadBalanceStrategy{}
	}

	c.JSON(http.StatusOK, strategies)
}

// getStrategy 获取策略详情
func (s *APIServer) getStrategy(c *gin.Context) {
	var strategy models.LoadBalanceStrategy
	if err := storage.DB.Where("id = ?", c.Param("id")).First(&strategy).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "策略不存在"})
		return
	}

	c.JSON(http.StatusOK, strategy)
}

type createStrategyRequest struct {
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	Direction   string            `json:"direction"`
	Metrics     []models.LBMetric `json:"metrics"`
}

// createStrategy 创建负载均衡策略
func (s *APIServer) createStrategy(c *gin.Context) {
	var req createStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metricsJSON, err := json.Marshal(req.Metrics)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "指标数据格式错误"})
		return
	}

	direction := req.Direction
	if direction != "desc" {
		direction = "asc"
	}

	strategy := models.LoadBalanceStrategy{
		ID:          utils.GenerateID("lbstrat"),
		Name:        req.Name,
		Description: req.Description,
		Direction:   direction,
		Metrics:     string(metricsJSON),
	}

	if err := storage.DB.Create(&strategy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logger.Info("创建负载均衡策略", zap.String("id", strategy.ID), zap.String("name", strategy.Name))
	c.JSON(http.StatusCreated, strategy)
}

type updateStrategyRequest struct {
	Name        *string           `json:"name"`
	Description *string           `json:"description"`
	Direction   *string           `json:"direction"`
	Metrics     []models.LBMetric `json:"metrics"`
}

// updateStrategy 更新负载均衡策略
func (s *APIServer) updateStrategy(c *gin.Context) {
	id := c.Param("id")

	var existing models.LoadBalanceStrategy
	if err := storage.DB.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "策略不存在"})
		return
	}

	var req updateStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Direction != nil {
		d := *req.Direction
		if d != "desc" {
			d = "asc"
		}
		updates["direction"] = d
	}
	if req.Metrics != nil {
		metricsJSON, err := json.Marshal(req.Metrics)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "指标数据格式错误"})
			return
		}
		updates["metrics"] = string(metricsJSON)
	}

	if len(updates) > 0 {
		if err := storage.DB.Model(&existing).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logger.Info("更新负载均衡策略", zap.String("id", id))
	}

	s.dispatcher.ClearStrategyCache(id)

	storage.DB.Where("id = ?", id).First(&existing)
	c.JSON(http.StatusOK, existing)
}

// deleteStrategy 删除负载均衡策略
func (s *APIServer) deleteStrategy(c *gin.Context) {
	id := c.Param("id")

	if err := storage.DB.Where("id = ?", id).Delete(&models.LoadBalanceStrategy{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 清除引用了该策略的 Job 的 strategy_id
	storage.DB.Model(&models.Job{}).Where("strategy_id = ?", id).Update("strategy_id", "")

	logger.Info("删除负载均衡策略", zap.String("id", id))
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// validateFormula 验证公式语法
func (s *APIServer) validateFormula(c *gin.Context) {
	var req struct {
		Formula string `json:"formula" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 formula 参数"})
		return
	}

	if err := ValidateFormula(req.Formula); err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"valid": true})
}

// getFormulaParameters 获取可用公式参数列表
func (s *APIServer) getFormulaParameters(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"parameters": FormulaParameterInfo})
}
