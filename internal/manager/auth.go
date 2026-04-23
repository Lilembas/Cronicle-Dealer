package manager

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/cronicle/cronicle-dealer/internal/models"
	"github.com/cronicle/cronicle-dealer/internal/storage"
	"github.com/cronicle/cronicle-dealer/pkg/logger"
	"github.com/cronicle/cronicle-dealer/pkg/utils"
)

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func (s *APIServer) login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}

	var user models.User
	if err := storage.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if !utils.BoolValue(user.Active) || !user.CheckPassword(req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token, err := s.generateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 token 失败"})
		return
	}

	now := time.Now()
	_ = storage.DB.Model(&user).Update("last_login_at", &now).Error

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":        user.ID,
			"username":  user.Username,
			"role":      user.Role,
			"full_name": user.FullName,
		},
	})
}

func (s *APIServer) refreshToken(c *gin.Context) {
	tokenStr, err := extractBearerToken(c.GetHeader("Authorization"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	claims, err := s.validateToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token 无效"})
		return
	}

	var user models.User
	if err := storage.DB.Where("id = ?", claims.UserID).First(&user).Error; err != nil || !utils.BoolValue(user.Active) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在或已禁用"})
		return
	}

	newToken, err := s.generateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "刷新 token 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": newToken})
}

func (s *APIServer) adminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role.(string) != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			return
		}
		c.Next()
	}
}

func (s *APIServer) userMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || (role.(string) != "admin" && role.(string) != "user") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要普通用户或管理员权限"})
			return
		}
		c.Next()
	}
}

func (s *APIServer) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := extractBearerToken(c.GetHeader("Authorization"))
		if err != nil {
			// 回退到 query param（用于 SSE 等场景）
			tokenStr = c.Query("token")
			if tokenStr == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
				return
			}
		}

		claims, err := s.validateToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token 无效或已过期"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func (s *APIServer) generateToken(user *models.User) (string, error) {
	expireHours := s.cfg.Security.JWT.ExpireHours
	if expireHours <= 0 {
		expireHours = 24
	}

	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.Security.JWT.Secret))
}

func (s *APIServer) validateToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(s.cfg.Security.JWT.Secret), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func extractBearerToken(header string) (string, error) {
	if header == "" {
		return "", errors.New("missing authorization header")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("invalid authorization header")
	}
	return parts[1], nil
}

func EnsureDefaultAdmin() error {
	var count int64
	if err := storage.DB.Model(&models.User{}).Where("username = ?", "admin").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	user := &models.User{
		ID:       "admin_default",
		Username: "admin",
		Email:    "admin@cronicle.local",
		Role:     "admin",
		Active:   utils.BoolPtr(true),
		FullName: "System Administrator",
	}
	if err := user.SetPassword("admin123"); err != nil {
		return err
	}

	if err := storage.DB.Create(user).Error; err != nil {
		return err
	}
	logger.Info("已创建默认管理员账号", zap.String("username", "admin"))
	return nil
}
