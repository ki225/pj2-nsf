package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

/*
TODO: Implement the customDb in this file.

You can change the file name if you want.

Make sure your db implement the DbIf interface in db.go.

Since the customDb will not test in TA's environment, you can just harcode the db's configuration at this file, e.g. db address/keys.
*/

const customDbTimeout = 30 * time.Second

type customDb struct {
	client *mongo.Client
	db     *mongo.Collection
}

var _ DbIf = (*customDb)(nil)

func newCustomDb(uri, dbName string) (*customDb, error) {
	if uri == "" {
		uri = "mongodb://127.0.0.1:27017"
	}
	if dbName == "" {
		dbName = "nsf"
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri).
		SetServerSelectionTimeout(customDbTimeout).SetTimeout(customDbTimeout))
	if err != nil {
		return nil, fmt.Errorf("connect MongoDB: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), customDbTimeout)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), customDbTimeout)
		defer cleanupCancel()
		_ = client.Disconnect(cleanupCtx)
		return nil, fmt.Errorf("ping MongoDB: %w", err)
	}
	return &customDb{
		client: client,
		db:     client.Database(dbName).Collection(serviceBucket),
	}, nil
}

func (c *customDb) Release() error {
	if c.client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), customDbTimeout)
	defer cancel()
	return c.client.Disconnect(ctx)
}

func (c *customDb) Exist(key string) (bool, error) {
	err := c.db.FindOne(context.Background(), bson.M{"_id": key}).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	return err == nil, err
}

func (c *customDb) Save(key, value string) error {
	_, err := c.db.ReplaceOne(context.Background(), bson.M{"_id": key},
		bson.M{"_id": key, "value": value}, options.Replace().SetUpsert(true))
	return err
}

// SaveAll uses ordered upserts. Unlike bbolt's transaction, a failed batch may
// have already saved some documents on a standalone MongoDB server.
func (c *customDb) SaveAll(data map[string]string) error {
	if len(data) == 0 {
		return nil
	}
	writes := make([]mongo.WriteModel, 0, len(data))
	for key, value := range data {
		writes = append(writes, mongo.NewReplaceOneModel().
			SetFilter(bson.M{"_id": key}).
			SetReplacement(bson.M{"_id": key, "value": value}).SetUpsert(true))
	}
	_, err := c.db.BulkWrite(context.Background(), writes)
	return err
}

func (c *customDb) Load(key string) (string, error) {
	var document struct {
		Value string `bson:"value"`
	}
	err := c.db.FindOne(context.Background(), bson.M{"_id": key}).Decode(&document)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return document.Value, nil
}

func (c *customDb) LoadAll() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), customDbTimeout)
	defer cancel()
	cursor, err := c.db.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()
	result := make(map[string]string)
	for cursor.Next(ctx) {
		var document struct {
			Key   string `bson:"_id"`
			Value string `bson:"value"`
		}
		if err := cursor.Decode(&document); err != nil {
			return nil, err
		}
		result[document.Key] = document.Value
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *customDb) Remove(key string) error {
	_, err := c.db.DeleteOne(context.Background(), bson.M{"_id": key})
	return err
}

func (c *customDb) RemoveAll() error {
	_, err := c.db.DeleteMany(context.Background(), bson.M{})
	return err
}
