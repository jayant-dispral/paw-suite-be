package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	"github.com/jayant-dispral/brand-threat-be/internal/core/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoUserRepository struct {
	coll *mongo.Collection
}

// NewUserRepository create the repo and ensures there is an index
func NewUserRepository(db *mongo.Database) ports.UserRepository {
	coll := db.Collection("users")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "email", Value: 1}}, //1-asending
			Options: options.Index().SetUnique(true),
		})
		if err != nil {
			panic("failed to create a unique index on users: " + err.Error())
		}
	}()

	return &MongoUserRepository{
		coll: coll,
	}
}

func (r *MongoUserRepository) Save(ctx context.Context, user domain.User) (string, error) {
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	res, err := r.coll.InsertOne(ctx, user)
	if err != nil {
		return "", err
	}

	oid, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", errors.New("failed to convert objectid")
	}
	return oid.Hex(), nil
}

func (r *MongoUserRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.coll.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (r *MongoUserRepository) GetUserById(ctx context.Context, id string) (*domain.User, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid id format")
	}

	var user domain.User
	err = r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}
