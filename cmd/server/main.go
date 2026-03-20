package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func authMiddleware(sessions *SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := tokenFromRequest(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "통행증이 없습니다"})
			c.Abort()
			return
		}
		user, ok := sessions.lookup(token)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "유효하지 않은 통행증입니다"})
			c.Abort()
			return
		}

		c.Set("user", user)
		c.Next()
	}
}

func registerStaticRoutes(router *gin.Engine) {
	router.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/static/") || c.Request.URL.Path == "/" {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}
		c.Next()
	})
	router.Static("/static", "./static")
	router.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})
}

func main() {
	store, err := openStore("./app.db", "./schema.sql", "./seed.sql")
	if err != nil {
		panic(err)
	}
	defer store.close()

	sessions := newSessionStore()

	authService := &AuthService{store: store, sessions: sessions}
	authHandler := &AuthHandler{service: authService, sessions: sessions}

	bankingService := &BankingService{store: store, sessions: sessions}
	bankingHandler := &BankingHandler{service: bankingService}

	postService := &PostService{store: store}
	postHandler := &PostHandler{service: postService}

	router := gin.Default()
	registerStaticRoutes(router)

	auth := router.Group("/api/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)

		authProtected := auth.Group("")
		authProtected.Use(authMiddleware(sessions))
		authProtected.POST("/logout", authHandler.Logout)
		authProtected.POST("/withdraw", authHandler.Withdraw)
	}

	protected := router.Group("/api")
	protected.Use(authMiddleware(sessions))
	{
		protected.GET("/me", func(c *gin.Context) {
			userObj, _ := c.Get("user")
			user := userObj.(User)
			c.JSON(http.StatusOK, gin.H{"user": makeUserResponse(user)})
		})

		protected.POST("/banking/deposit", bankingHandler.Deposit)
		protected.POST("/banking/withdraw", bankingHandler.Withdraw)
		protected.POST("/banking/transfer", bankingHandler.Transfer)

		protected.GET("/posts", postHandler.GetPosts)
		protected.POST("/posts", postHandler.CreatePost)
		protected.GET("/posts/:id", postHandler.GetPostByID)
		protected.PUT("/posts/:id", postHandler.UpdatePost)
		protected.DELETE("/posts/:id", postHandler.DeletePost)
	}

	if err := router.Run(":8080"); err != nil {
		panic(err)
	}
}
