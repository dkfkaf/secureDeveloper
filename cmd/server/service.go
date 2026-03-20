package main

import (
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	store    *Store
	sessions *SessionStore
}

func (s *AuthService) Register(req RegisterRequest) error {
	if err := validateUsername(req.Username); err != nil {
		return err
	}
	if err := validatePassword(req.Password); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("비밀번호 암호화에 실패했습니다")
	}

	return s.store.CreateUser(req.Username, req.Name, req.Email, req.Phone, string(hashedPassword))
}

func (s *AuthService) Login(username, password string) (string, User, error) {
	user, ok, err := s.store.FindUserByUsername(username)
	if err != nil || !ok {
		return "", User{}, errors.New("아이디 또는 비밀번호가 올바르지 않습니다")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return "", User{}, errors.New("아이디 또는 비밀번호가 올바르지 않습니다")
	}

	token, err := s.sessions.create(user)
	if err != nil {
		return "", User{}, errors.New("세션 생성 실패")
	}
	return token, user, nil
}

func (s *AuthService) WithdrawAccount(user User, password, token string) error {
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return errors.New("비밀번호가 올바르지 않습니다")
	}

	err = s.store.DeleteUser(user.ID)
	if err != nil {
		return errors.New("계정 삭제 실패")
	}

	s.sessions.delete(token)
	return nil
}

type BankingService struct {
	store    *Store
	sessions *SessionStore
}

func (s *BankingService) Deposit(user User, amount int64, token string) (User, error) {
	err := s.store.UpdateBalance(user.ID, amount)
	if err != nil {
		return user, errors.New("입금 처리 실패")
	}
	user.Balance += amount
	s.sessions.update(token, user)
	return user, nil
}

func (s *BankingService) Withdraw(user User, amount int64, token string) (User, error) {
	if user.Balance < amount {
		return user, errors.New("잔액이 부족합니다")
	}
	err := s.store.UpdateBalance(user.ID, -amount)
	if err != nil {
		return user, errors.New("출금 처리 실패")
	}
	user.Balance -= amount
	s.sessions.update(token, user)
	return user, nil
}

func (s *BankingService) Transfer(user User, toUsername string, amount int64, token string) (User, error) {
	err := s.store.TransferBalance(user.ID, toUsername, amount)
	if err != nil {
		return user, err
	}
	user.Balance -= amount
	s.sessions.update(token, user)
	return user, nil
}

type PostService struct {
	store *Store
}

func (s *PostService) GetPosts() ([]PostView, error) {
	return s.store.GetPosts()
}

func (s *PostService) CreatePost(user User, title, content string) (PostView, error) {
	now := time.Now().Format(time.RFC3339)
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)

	postID, err := s.store.CreatePost(title, content, user.ID, user.Name, user.Email, now)
	if err != nil {
		return PostView{}, errors.New("게시글 생성 실패")
	}

	return PostView{
		ID:          uint(postID),
		Title:       title,
		Content:     content,
		OwnerID:     user.ID,
		Author:      user.Name,
		AuthorEmail: user.Email,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (s *PostService) GetPostByID(id string) (PostView, error) {
	return s.store.GetPostByID(id)
}

func (s *PostService) UpdatePost(id string, user User, title, content string) error {
	now := time.Now().Format(time.RFC3339)
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)

	rowsAffected, err := s.store.UpdatePost(id, user.ID, title, content, now)
	if err != nil {
		return errors.New("게시글 업데이트 실패")
	}
	if rowsAffected == 0 {
		return errors.New("수정 권한이 없거나 게시글이 존재하지 않습니다")
	}
	return nil
}

func (s *PostService) DeletePost(id string, user User) error {
	rowsAffected, err := s.store.DeletePost(id, user.ID)
	if err != nil {
		return errors.New("게시글 삭제 실패")
	}
	if rowsAffected == 0 {
		return errors.New("삭제 권한이 없거나 게시글이 존재하지 않습니다")
	}
	return nil
}
