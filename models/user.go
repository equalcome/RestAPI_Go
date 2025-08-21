package models

import (
	"errors"
	"restapi/db"
	"restapi/utils"
)

type User struct {
	ID       int64
	Email    string `binding:"required"`
	Password string `binding:"required"`
}

func (u *User) Save() error {
	query := "INSERT INTO users(email, password) VALUES (?, ?)" //這時候才會產生id
	stmt, err := db.DB.Prepare(query)
	if err != nil {
		return err
	}

	defer stmt.Close()

	hashedPassword, err := utils.HashPasseord(u.Password)
	if err != nil {
		return err
	}

	result, err := stmt.Exec(u.Email, hashedPassword) //存入db
	if err != nil {
		return err
	}

	userId, err := result.LastInsertId()
	u.ID = userId
	return err
}

// [HTTP 請求 JSON] → signup() → ShouldBindJSON 填入 User struct → 
// User.Save() (進入db)→ INSERT INTO users → DB 產生 id → u.ID = 新 id


func (u *User) ValidateCredentials() error {
		query := "SELECT id, password FROM users WHERE email = ?"
		row := db.DB.QueryRow(query, u.Email)

		var retrievedPassword string //db拿出來的哈希密碼
		err := row.Scan(&u.ID, &retrievedPassword) //把查詢到的 password(SELECT password) 欄位值填入 retrievedPassword
												   //ID也要取出來(SELECT id, password)，在放在物件裡
		if err != nil { //if no rows match the query 回傳 error
			return errors.New("Credentials invalid")
		}

		//	retrievedPassword（資料庫的雜湊值）去比對 u.Password（使用者輸入的明碼）
		passwordIsValid := utils.CheckPasswordHash(u.Password, retrievedPassword)
		if !passwordIsValid {
			return errors.New("Credentials invalid")
		}

		return nil

	}
