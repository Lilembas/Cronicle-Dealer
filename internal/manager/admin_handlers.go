package manager

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
	"github.com/cronicle/cronicle-next/pkg/utils"
)

func (s *APIServer) listUsers(c *gin.Context) {
	var users []models.User
	query := storage.DB

	if role := c.Query("role"); role != "" {
		query = query.Where("role = ?", role)
	}
	if active := c.Query("active"); active != "" {
		query = query.Where("active = ?", active == "true")
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	offset := (page - 1) * pageSize

	var total int64
	query.Model(&models.User{}).Count(&total)

	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"page":  page,
		"data":  users,
	})
}

func (s *APIServer) getUser(c *gin.Context) {
	var user models.User
	if err := storage.DB.Where("id = ?", c.Param("id")).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (s *APIServer) createUser(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		FullName string `json:"full_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if req.Role == "" {
		req.Role = "user"
	}
	if req.Role != "admin" && req.Role != "user" && req.Role != "viewer" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的角色类型"})
		return
	}

	var count int64
	storage.DB.Model(&models.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	}

	user := &models.User{
		ID:       utils.GenerateID("usr"),
		Username: req.Username,
		Email:    req.Email,
		Role:     req.Role,
		FullName: req.FullName,
		Active:   utils.BoolPtr(true),
	}

	if err := user.SetPassword(req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	if err := storage.DB.Create(user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

func (s *APIServer) updateUser(c *gin.Context) {
	userID := c.Param("id")

	var existing models.User
	if err := storage.DB.Where("id = ?", userID).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	var req struct {
		Username *string `json:"username"`
		Email    *string `json:"email"`
		Role     *string `json:"role"`
		FullName *string `json:"full_name"`
		Active   *bool   `json:"active"`
		Password *string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Username != nil && *req.Username != existing.Username {
		var count int64
		storage.DB.Model(&models.User{}).Where("username = ? AND id != ?", *req.Username, userID).Count(&count)
		if count > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
			return
		}
		updates["username"] = *req.Username
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Role != nil {
		if *req.Role != "admin" && *req.Role != "user" && *req.Role != "viewer" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的角色类型"})
			return
		}
		updates["role"] = *req.Role
	}
	if req.FullName != nil {
		updates["full_name"] = *req.FullName
	}
	if req.Active != nil {
		if !*req.Active && existing.Role == "admin" {
			var adminCount int64
			storage.DB.Model(&models.User{}).Where("role = ? AND active = ?", "admin", true).Count(&adminCount)
			if adminCount <= 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "不能禁用最后一个管理员"})
				return
			}
		}
		updates["active"] = *req.Active
	}
	if req.Password != nil && *req.Password != "" {
		if len(*req.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "密码长度不能少于6位"})
			return
		}
		if err := existing.SetPassword(*req.Password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
			return
		}
		updates["password"] = existing.Password
	}

	if len(updates) > 0 {
		if err := storage.DB.Model(&existing).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户失败: " + err.Error()})
			return
		}
	}

	storage.DB.Where("id = ?", userID).First(&existing)
	c.JSON(http.StatusOK, existing)
}

func (s *APIServer) deleteUser(c *gin.Context) {
	userID := c.Param("id")

	currentUserID, _ := c.Get("user_id")
	if userID == currentUserID.(string) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除自己"})
		return
	}

	var user models.User
	if err := storage.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if user.Role == "admin" {
		var adminCount int64
		storage.DB.Model(&models.User{}).Where("role = ?", "admin").Count(&adminCount)
		if adminCount <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除最后一个管理员"})
			return
		}
	}

	if err := storage.DB.Where("id = ?", userID).Delete(&models.User{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除用户失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func (s *APIServer) changePassword(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	var user models.User
	if err := storage.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if !user.CheckPassword(req.OldPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "旧密码错误"})
		return
	}

	if err := user.SetPassword(req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	if err := storage.DB.Model(&user).Update("password", user.Password).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}
