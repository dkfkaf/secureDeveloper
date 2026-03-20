package main

import (
	"database/sql"
	"errors"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func openStore(databasePath, schemaFile, seedFile string) (*Store, error) {
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.initialize(schemaFile, seedFile); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) close() error {
	return s.db.Close()
}

func (s *Store) initialize(schemaFile, seedFile string) error {
	if err := s.execSQLFile(schemaFile); err != nil {
		return err
	}
	if err := s.execSQLFile(seedFile); err != nil {
		return err
	}
	return nil
}

func (s *Store) execSQLFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(string(content))
	return err
}

func (s *Store) CreateUser(username, name, email, phone, hashedPassword string) error {
	_, err := s.db.Exec(`
		INSERT INTO users (username, name, email, phone, password, balance, is_admin)
		VALUES (?, ?, ?, ?, ?, 0, 0)
	`, username, name, email, phone, hashedPassword)
	return err
}

func (s *Store) FindUserByUsername(username string) (User, bool, error) {
	row := s.db.QueryRow(`
		SELECT id, username, name, email, phone, password, balance, is_admin
		FROM users
		WHERE username = ?
	`, strings.TrimSpace(username))

	var user User
	var isAdmin int64
	if err := row.Scan(&user.ID, &user.Username, &user.Name, &user.Email, &user.Phone, &user.Password, &user.Balance, &isAdmin); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, false, nil
		}
		return User{}, false, err
	}
	user.IsAdmin = isAdmin == 1
	return user, true, nil
}

func (s *Store) DeleteUser(userID uint) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, userID)
	return err
}

func (s *Store) UpdateBalance(userID uint, amount int64) error {
	_, err := s.db.Exec(`UPDATE users SET balance = balance + ? WHERE id = ?`, amount, userID)
	return err
}

func (s *Store) TransferBalance(fromUserID uint, toUsername string, amount int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentBalance int64
	err = tx.QueryRow(`SELECT balance FROM users WHERE id = ?`, fromUserID).Scan(&currentBalance)
	if err != nil {
		return err
	}
	if currentBalance < amount {
		return errors.New("insufficient balance")
	}

	_, err = tx.Exec(`UPDATE users SET balance = balance - ? WHERE id = ?`, amount, fromUserID)
	if err != nil {
		return err
	}

	res, err := tx.Exec(`UPDATE users SET balance = balance + ? WHERE username = ?`, amount, toUsername)
	if err != nil {
		return err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return errors.New("target user not found")
	}

	return tx.Commit()
}

func (s *Store) GetPosts() ([]PostView, error) {
	rows, err := s.db.Query(`SELECT id, title, content, owner_id, author, author_email, created_at, updated_at FROM posts ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []PostView
	for rows.Next() {
		var p PostView
		if err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.OwnerID, &p.Author, &p.AuthorEmail, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		posts = append(posts, p)
	}
	if posts == nil {
		posts = []PostView{}
	}
	return posts, nil
}

func (s *Store) CreatePost(title, content string, ownerID uint, author, authorEmail, now string) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO posts (title, content, owner_id, author, author_email, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, title, content, ownerID, author, authorEmail, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) GetPostByID(id string) (PostView, error) {
	row := s.db.QueryRow(`SELECT id, title, content, owner_id, author, author_email, created_at, updated_at FROM posts WHERE id = ?`, id)
	var p PostView
	err := row.Scan(&p.ID, &p.Title, &p.Content, &p.OwnerID, &p.Author, &p.AuthorEmail, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s *Store) UpdatePost(id string, ownerID uint, title, content, now string) (int64, error) {
	res, err := s.db.Exec(`
		UPDATE posts SET title = ?, content = ?, updated_at = ?
		WHERE id = ? AND owner_id = ?
	`, title, content, now, id, ownerID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) DeletePost(id string, ownerID uint) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM posts WHERE id = ? AND owner_id = ?`, id, ownerID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
