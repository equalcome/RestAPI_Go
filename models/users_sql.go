package models

import (
	"database/sql"
	"errors"
	"restapi/utils" // 這裡假設你在 utils 裡有 HashPassword / CheckPasswordHash
)

type sqlUserRepo struct{ db *sql.DB } //真db 下面做他的與db的操作 //實現介面方法

func NewSQLUserRepository(db *sql.DB) UserRepository { return &sqlUserRepo{db} }

func (r *sqlUserRepo) Create(u *User) error {
	// 假設 u.Password 目前是 plain text → 先雜湊
	hashed, err := utils.HashPassword(u.Password)
	if err != nil {
		return err
	}
	u.Password = hashed

	_, err = r.db.Exec(`INSERT INTO users(email, password) VALUES ($1,$2)`, u.Email, u.Password)
	return err
}

func (r *sqlUserRepo) ValidateCredentials(email, plain string) (User, error) {
	var u User
	err := r.db.QueryRow(`SELECT id, email, password FROM users WHERE email=$1`, email).
		Scan(&u.ID, &u.Email, &u.Password)
	if err != nil {
		return User{}, err
	}

	// 用 bcrypt 比對 plain vs hashed
	if !utils.CheckPasswordHash(plain, u.Password) {
		return User{}, errors.New("invalid credentials")
	}

	return u, nil
}

func (r *sqlUserRepo) GetByID(id int64) (User, error) {
	var u User
	err := r.db.QueryRow(`SELECT id, email FROM users WHERE id=$1`, id).
		Scan(&u.ID, &u.Email)
	if err != nil {
		return User{}, err
	}
	return u, nil
}
