package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func tokenFromRequest(c *gin.Context) string {
	headerValue := strings.TrimSpace(c.GetHeader("Authorization"))
	if headerValue != "" {
		return headerValue
	}
	cookieValue, err := c.Cookie(authorizationCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookieValue)
}

func clearAuthorizationCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authorizationCookieName, "", -1, "/", "", false, true)
}

type AuthHandler struct {
	service  *AuthService
	sessions *SessionStore
}

func (h *AuthHandler) Register(c *gin.Context) {
	var request RegisterRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "잘못된 요청입니다"})
		return
	}

	if err := h.service.Register(request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "회원가입 완료",
		"username": request.Username,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var request LoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "잘못된 요청입니다"})
		return
	}

	token, user, err := h.service.Login(request.Username, request.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": err.Error()})
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authorizationCookieName, token, 60*60*8, "/", "", false, true)
	c.JSON(http.StatusOK, LoginResponse{
		AuthMode: "header-and-cookie",
		Token:    token,
		User:     makeUserResponse(user),
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	token := tokenFromRequest(c)
	h.sessions.delete(token)
	clearAuthorizationCookie(c)

	// 로그아웃 감사 로직 추가 예정
	c.JSON(http.StatusOK, gin.H{"message": "로그아웃 성공"})
}

func (h *AuthHandler) Withdraw(c *gin.Context) {
	var request WithdrawAccountRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "잘못된 요청입니다"})
		return
	}

	userObj, _ := c.Get("user")
	user := userObj.(User)
	token := tokenFromRequest(c)

	err := h.service.WithdrawAccount(user, request.Password, token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": err.Error()})
		return
	}

	clearAuthorizationCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "계정 삭제 성공"})
}

type BankingHandler struct {
	service *BankingService
}

func (h *BankingHandler) Deposit(c *gin.Context) {
	var request DepositRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "잘못된 요청입니다"})
		return
	}
	if request.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "금액은 1원 이상이어야 합니다."})
		return
	}

	userObj, _ := c.Get("user")
	user := userObj.(User)
	token := tokenFromRequest(c)

	updatedUser, err := h.service.Deposit(user, request.Amount, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "입금 성공",
		"user":    makeUserResponse(updatedUser),
		"amount":  request.Amount,
	})
}

func (h *BankingHandler) Withdraw(c *gin.Context) {
	var request BalanceWithdrawRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "잘못된 요청입니다"})
		return
	}
	if request.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "금액은 1원 이상이어야 합니다."})
		return
	}

	userObj, _ := c.Get("user")
	user := userObj.(User)
	token := tokenFromRequest(c)

	updatedUser, err := h.service.Withdraw(user, request.Amount, token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "출금 성공",
		"user":    makeUserResponse(updatedUser),
		"amount":  request.Amount,
	})
}

func (h *BankingHandler) Transfer(c *gin.Context) {
	var request TransferRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "잘못된 요청입니다"})
		return
	}
	if request.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "금액은 1원 이상이어야 합니다."})
		return
	}

	userObj, _ := c.Get("user")
	user := userObj.(User)
	token := tokenFromRequest(c)

	updatedUser, err := h.service.Transfer(user, request.ToUsername, request.Amount, token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "송금 성공",
		"user":    makeUserResponse(updatedUser),
		"amount":  request.Amount,
	})
}

type PostHandler struct {
	service *PostService
}

func (h *PostHandler) GetPosts(c *gin.Context) {
	posts, err := h.service.GetPosts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, PostListResponse{Posts: posts})
}

func (h *PostHandler) CreatePost(c *gin.Context) {
	var request CreatePostRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "잘못된 요청입니다"})
		return
	}

	userObj, _ := c.Get("user")
	user := userObj.(User)

	post, err := h.service.CreatePost(user, request.Title, request.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "게시글 작성 성공",
		"post":    post,
	})
}

func (h *PostHandler) GetPostByID(c *gin.Context) {
	postID := c.Param("id")
	post, err := h.service.GetPostByID(postID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "게시글을 찾을 수 없습니다"})
		return
	}
	c.JSON(http.StatusOK, PostResponse{Post: post})
}

func (h *PostHandler) UpdatePost(c *gin.Context) {
	var request UpdatePostRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "잘못된 요청입니다"})
		return
	}

	userObj, _ := c.Get("user")
	user := userObj.(User)
	postID := c.Param("id")

	err := h.service.UpdatePost(postID, user, request.Title, request.Content)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "게시글 수정 성공"})
}

func (h *PostHandler) DeletePost(c *gin.Context) {
	userObj, _ := c.Get("user")
	user := userObj.(User)
	postID := c.Param("id")

	err := h.service.DeletePost(postID, user)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "게시글 삭제 성공"})
}
