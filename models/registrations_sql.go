package models

import "database/sql"

type sqlRegistrationRepo struct{ db *sql.DB }

func NewSQLRegistrationRepository(db *sql.DB) RegistrationRepository {
    return &sqlRegistrationRepo{db}
}

func (r *sqlRegistrationRepo) Register(userID int64, eventID string) error {
    // 依賴 UNIQUE(user_id, event_id) 來杜絕重複
    _, err := r.db.Exec(`INSERT INTO registrations(user_id, event_id) VALUES ($1,$2)`, userID, eventID)
    return err
}

func (r *sqlRegistrationRepo) Cancel(userID int64, eventID string) error {
    _, err := r.db.Exec(`DELETE FROM registrations WHERE user_id=$1 AND event_id=$2`, userID, eventID)
    return err
}
