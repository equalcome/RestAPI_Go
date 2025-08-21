# Event Management API

A RESTful API built with **Go (Golang)** and **Gin framework** for managing events and user registrations.  
It supports authentication with **JWT**, password hashing, and CRUD operations for events.

---

## 🚀 Features

- User authentication with **JWT**
- Passwords secured using **bcrypt hashing**
- **SQLite database** (auto-created with tables for users, events, registrations)
- Full **CRUD operations** for events
- Register and cancel registration for events
- Only event creators can **update** or **delete** their events
- Middleware to verify JWT before accessing protected routes

---

## 🔑 API Endpoints

| Method | Endpoint                  | Description                     | Auth Required | Notes                  |
|--------|---------------------------|---------------------------------|---------------|------------------------|
| GET    | `/events`                 | Get all events                  | No            |                        |
| GET    | `/events/:id`             | Get event by ID                 | No            |                        |
| POST   | `/events`                 | Create a new event              | Yes           |                        |
| PUT    | `/events/:id`             | Update an event                 | Yes           | Only creator can edit  |
| DELETE | `/events/:id`             | Delete an event                 | Yes           | Only creator can delete|
| POST   | `/signup`                 | Register a new user             | No            |                        |
| POST   | `/login`                  | Authenticate user (JWT issued)  | No            | Returns JWT token      |
| POST   | `/events/:id/register`    | Register user for an event      | Yes           |                        |
| DELETE | `/events/:id/register`    | Cancel event registration       | Yes           |                        |
