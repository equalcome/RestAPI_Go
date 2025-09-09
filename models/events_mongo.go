package models

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type mongoEventRepo struct {
    col *mongo.Collection
}

func NewMongoEventRepository(col *mongo.Collection) EventRepository {
    return &mongoEventRepo{col: col}
}

func (r *mongoEventRepo) GetAll() ([]Event, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    cur, err := r.col.Find(ctx, bson.M{})
    if err != nil { return nil, err }
    defer cur.Close(ctx)

    var out []Event
    for cur.Next(ctx) {
        var e Event
        if err := cur.Decode(&e); err != nil { return nil, err }
        out = append(out, e)
    }
    return out, cur.Err()
}

func (r *mongoEventRepo) GetByID(id string) (Event, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    var e Event
    if err := r.col.FindOne(ctx, bson.M{"id": id}).Decode(&e); err != nil {
        if errors.Is(err, mongo.ErrNoDocuments) { return Event{}, err }
        return Event{}, err
    }
    return e, nil
}

func (r *mongoEventRepo) Create(e *Event) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _, err := r.col.InsertOne(ctx, e)
    return err
}

func (r *mongoEventRepo) Update(e *Event) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _, err := r.col.UpdateOne(ctx, bson.M{"id": e.ID}, bson.M{"$set": e})
    return err
}

func (r *mongoEventRepo) Delete(id string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _, err := r.col.DeleteOne(ctx, bson.M{"id": id})
    return err
}
