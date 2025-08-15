package docs

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = struct {
	Version     string
	Host        string
	BasePath    string
	Schemes     []string
	Title       string
	Description string
}{
	Version:     "1.0",
	Host:        "localhost:8080",
	BasePath:    "/api/v1",
	Schemes:     []string{"http", "https"},
	Title:       "Go Web Starter API",
	Description: "这是一个基于Gin框架的Web应用启动模板的API文档",
}

// SetupSwagger 设置Swagger文档路由
func SetupSwagger(r *gin.Engine) {
	// Swagger文档路由
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API文档重定向
	r.GET("/docs", func(c *gin.Context) {
		c.Redirect(302, "/swagger/index.html")
	})

	// API文档首页
	r.GET("/api-docs", func(c *gin.Context) {
		c.Redirect(302, "/swagger/index.html")
	})
}

// UpdateSwaggerInfo 更新Swagger信息
func UpdateSwaggerInfo(version, host, basePath, title, description string) {
	if version != "" {
		SwaggerInfo.Version = version
	}
	if host != "" {
		SwaggerInfo.Host = host
	}
	if basePath != "" {
		SwaggerInfo.BasePath = basePath
	}
	if title != "" {
		SwaggerInfo.Title = title
	}
	if description != "" {
		SwaggerInfo.Description = description
	}
}
