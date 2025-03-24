package router

import (
	"fmt"

	"github.com/Masterminds/sprig/v3"
	"github.com/gin-gonic/gin"

	"github.com/narasux/goblog/pkg/envs"
	"github.com/narasux/goblog/pkg/handler"
	"github.com/narasux/goblog/pkg/middleware"
)

func InitRouter() {
	gin.SetMode(envs.GinRunMode)
	router := gin.New()
	_ = router.SetTrustedProxies(nil)

	router.Use(middleware.RequestID())
	router.Use(middleware.Logger())
	router.Use(middleware.Cors())
	router.Use(gin.Recovery())

	// 设置静态文件
	router.Static("/static", envs.StaticFileBaseDir)
	// 设置模板方法
	router.SetFuncMap(sprig.FuncMap())
	// 加载 HTML 模板文件
	router.LoadHTMLGlob(envs.TmplFileBaseDir + "/webfe/*")
	// 404
	router.NoRoute(handler.Get404)
	// robots.txt
	router.GET("robots.txt", handler.GetRobotsTxt)

	// webfe 路由
	{
		webfeRg := router.Group("")
		// 主页
		webfeRg.GET("", handler.GetHomePage)
		webfeRg.GET("home", handler.GetHomePage)
		// 博客文章列表
		webfeRg.GET("articles", handler.ListArticles)
		// 博客文章详情
		webfeRg.GET("articles/:id", handler.RetrieveArticle)
		// RSS
		webfeRg.GET("rss", handler.GetRSS)
	}

	// api 路由
	{
		apiRg := router.Group("apis")
		// 点赞博客文章
		apiRg.POST("articles/:id/like", handler.LikeArticle)
	}

	if err := router.Run(":" + envs.ServerPort); err != nil {
		panic(fmt.Sprintf("failed to start server: %s", err.Error()))
	}
}
