package mongo

import (
	"context"
	"os"
	"time"

	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var client *mongodriver.Client

// Init initializes a global MongoDB client using MONGO_URI.
// Safe to call multiple times; repeated calls are no-ops if already connected.
func Init() error {
	if client != nil {
		return nil
	}

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := mongodriver.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}
	if err := c.Ping(ctx, readpref.Primary()); err != nil {
		_ = c.Disconnect(context.Background())
		return err
	}

	client = c
	return nil
}

// Client returns the initialized Mongo client.
func Client() *mongodriver.Client { return client }

// Close disconnects the global client if present.
func Close(ctx context.Context) error {
	if client == nil {
		return nil
	}
	err := client.Disconnect(ctx)
	client = nil
	return err
}
