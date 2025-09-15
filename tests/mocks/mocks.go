package mocks

import (
	"errors"
	"fmt"
	"restapi/models"
)

type MockUserRepo struct {
	Users map[string]models.User // key 是 email  //假db 下面做他的與db的操作 //實現介面方法
}
func (m *MockUserRepo) Create(u *models.User) error {
	if _, ok := m.Users[u.Email]; ok { return errors.New("dup") }
	u.ID = int64(len(m.Users) + 1)
	m.Users[u.Email] = *u
	return nil
}
func (m *MockUserRepo) ValidateCredentials(email, plain string) (models.User, error) {
	u, ok := m.Users[email]; if !ok { return models.User{}, errors.New("not found") }
	// 測試先簡化：直接用明碼比對；之後可改成 utils.CheckPasswordHash
	if u.Password != plain { return models.User{}, errors.New("bad") }
	return u, nil
}
func (m *MockUserRepo) GetByID(id int64) (models.User, error) {
	for _, u := range m.Users { if u.ID == id { return u, nil } }
	return models.User{}, errors.New("not found")
}

type MockEventRepo struct{ Items map[string]models.Event }
func (m *MockEventRepo) GetAll() ([]models.Event, error) {
	out := make([]models.Event, 0, len(m.Items))
	for _, e := range m.Items { out = append(out, e) }
	return out, nil
}
func (m *MockEventRepo) GetByID(id string) (models.Event, error) {
	e, ok := m.Items[id]; if !ok { return models.Event{}, errors.New("nf") }
	return e, nil
}
func (m *MockEventRepo) Create(e *models.Event) error { m.Items[e.ID] = *e; return nil }
func (m *MockEventRepo) Update(e *models.Event) error {
	if _, ok := m.Items[e.ID]; !ok { return errors.New("nf") }
	m.Items[e.ID] = *e; return nil
}
func (m *MockEventRepo) Delete(id string) error { delete(m.Items, id); return nil }

type MockRegRepo struct{ Pairs map[string]bool } // "userId:eventId"
func (m *MockRegRepo) Register(uid int64, eid string) error {
	k := key(uid, eid); if m.Pairs[k] { return errors.New("dup") }
	m.Pairs[k] = true; return nil
}
func (m *MockRegRepo) Cancel(uid int64, eid string) error {
	delete(m.Pairs, key(uid, eid)); return nil
}
func key(uid int64, eid string) string { return fmt.Sprintf("%d:%s", uid, eid) }
