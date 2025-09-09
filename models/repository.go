package models

import "time"

type Event struct {
    ID          string    `json:"id"` // 使用 UUID（跨庫統一鍵）
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Location    string    `json:"location"`
    DateTime    time.Time `json:"dateTime"`
    UserID      int64     `json:"userId"` // 建立者（來自 SQL Users）
}

// ===== Events =====
type EventRepository interface {  //就把它當成一個struct 可以接收任何實體化它方法的物件   var a EventRepository = 
    GetAll() ([]Event, error)
    GetByID(id string) (Event, error)
    Create(e *Event) error
    Update(e *Event) error
    Delete(id string) error
}

// ===== Users（維持你原本邏輯）=====
type User struct {
    ID       int64  `json:"id"`
    Email    string `json:"email"`
    Password string `json:"password"`
}
type UserRepository interface {
    Create(u *User) error
    ValidateCredentials(email, plain string) (User, error)
    GetByID(id int64) (User, error)
}

// ===== Registrations =====
type RegistrationRepository interface {
    Register(userID int64, eventID string) error
    Cancel(userID int64, eventID string) error
    // 需要的話：ListByUser(userID), ListByEvent(eventID)...
}
