package manager

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cronicle/cronicle-dealer/internal/models"
	"github.com/cronicle/cronicle-dealer/internal/storage"
	"github.com/cronicle/cronicle-dealer/pkg/utils"
)

type categoryWithCount struct {
	models.Category
	JobCount int64 `json:"job_count"`
}

func (s *APIServer) listCategories(c *gin.Context) {
	var categories []models.Category
	storage.DB.Order("name ASC").Find(&categories)

	if len(categories) == 0 {
		defaultCat := models.Category{
			ID:   utils.GenerateID("cat"),
			Name: "默认分组",
		}
		storage.DB.Create(&defaultCat)
		categories = append(categories, defaultCat)
	}

	var jobCategories []string
	storage.DB.Model(&models.Job{}).Where("category != '' AND category IS NOT NULL").Distinct("category").Pluck("category", &jobCategories)

	catMap := make(map[string]int, len(categories))
	for i, cat := range categories {
		catMap[cat.Name] = i
	}

	now := time.Now()
	for _, name := range jobCategories {
		if _, exists := catMap[name]; !exists {
			cat := models.Category{
				ID:        utils.GenerateID("cat"),
				Name:      name,
				CreatedAt: now,
				UpdatedAt: now,
			}
			storage.DB.Create(&cat)
			categories = append(categories, cat)
			catMap[name] = len(categories) - 1
		}
	}

	result := make([]categoryWithCount, 0, len(categories))
	for _, cat := range categories {
		var count int64
		storage.DB.Model(&models.Job{}).Where("category = ?", cat.Name).Count(&count)
		result = append(result, categoryWithCount{
			Category: cat,
			JobCount: count,
		})
	}

	c.JSON(http.StatusOK, result)
}

func (s *APIServer) createCategory(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分组名称不能为空"})
		return
	}

	var count int64
	storage.DB.Model(&models.Category{}).Where("name = ?", req.Name).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "分组名称已存在"})
		return
	}

	category := &models.Category{
		ID:   utils.GenerateID("cat"),
		Name: req.Name,
	}

	if err := storage.DB.Create(category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建分组失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, category)
}

func (s *APIServer) updateCategory(c *gin.Context) {
	categoryID := c.Param("id")

	var existing models.Category
	if err := storage.DB.Where("id = ?", categoryID).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分组不存在"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分组名称不能为空"})
		return
	}

	if req.Name == existing.Name {
		c.JSON(http.StatusOK, existing)
		return
	}

	var count int64
	storage.DB.Model(&models.Category{}).Where("name = ? AND id != ?", req.Name, categoryID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "分组名称已存在"})
		return
	}

	oldName := existing.Name
	if err := storage.DB.Model(&existing).Update("name", req.Name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新分组失败: " + err.Error()})
		return
	}

	storage.DB.Model(&models.Job{}).Where("category = ?", oldName).Update("category", req.Name)

	storage.DB.Where("id = ?", categoryID).First(&existing)
	c.JSON(http.StatusOK, existing)
}

func (s *APIServer) deleteCategory(c *gin.Context) {
	categoryID := c.Param("id")

	var existing models.Category
	if err := storage.DB.Where("id = ?", categoryID).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分组不存在"})
		return
	}

	var jobCount int64
	storage.DB.Model(&models.Job{}).Where("category = ?", existing.Name).Count(&jobCount)
	if jobCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该分组下还有任务，无法删除"})
		return
	}

	if err := storage.DB.Where("id = ?", categoryID).Delete(&models.Category{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除分组失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
