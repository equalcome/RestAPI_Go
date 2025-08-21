# Event Management API

A RESTful API built with **Go (Golang)** and **Gin framework** for managing events and user registrations.  
It supports authentication with **JWT**, password hashing, and CRUD operations for events.

---

## 🚀 Features

- **User Management**
  - Signup with hashed passwords (bcrypt)
  - Login with JWT authentication
- **Event Management**
  - Create, read, update, and delete events
  - Only event creators can update or delete their events
- **Event Registration**
  - Register for an event
  - Cancel registration
- **Security**
  - JWT-based authentication middleware
  - Protected endpoints for authorized users only
- **Database**
  - SQLite database auto-created with tables for users, events, and registrations

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
| POST   | `/login`                  | Authenticate user (JWT)         | No            | Returns JWT token      |
| POST   | `/events/:id/register`    | Register user for an event      | Yes           |                        |
| DELETE | `/events/:id/register`    | Cancel event registration       | Yes           |                        |
