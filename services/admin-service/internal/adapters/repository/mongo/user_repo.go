package mongo

import (
	"context"
	stderrors "errors"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/jayant-dispral/brand-threat-be/shared/pkg/errors"
	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
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
			log.Printf("failed to create a unique index on users: %v", err)
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
		if strings.Contains(err.Error(), "E11000") || strings.Contains(err.Error(), "duplicate") {
			return "", errors.NewError(domain.ErrConflict, err)
		}
		return "", errors.NewError(domain.ErrInternal, err)
	}

	oid, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", errors.NewError(domain.ErrInternal, stderrors.New("failed to convert objectid"))
	}
	return oid.Hex(), nil
}

func (r *MongoUserRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.coll.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.NewError(domain.ErrNotFound, err)
		}
		return nil, errors.NewError(domain.ErrInternal, err)
	}
	return &user, nil
}

func (r *MongoUserRepository) GetUserById(ctx context.Context, id string) (*domain.User, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.NewError(domain.ErrInvalidInput, err)
	}

	var user domain.User
	err = r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.NewError(domain.ErrNotFound, err)
		}
		return nil, errors.NewError(domain.ErrInternal, err)
	}
	return &user, nil
}

func (r *MongoUserRepository) UpdateUser(ctx context.Context, id string, update domain.UpdateUserStruct) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.NewError(domain.ErrNotFound, err)
	}
	updateDoc := buildUpdateDoc(update)
	if len(updateDoc) == 0 {
		return nil
	}

	updateDoc["updated_at"] = time.Now()
	filter := bson.M{"_id": oid}
	updateData := bson.M{"$set": updateDoc}

	res, err := r.coll.UpdateOne(ctx, filter, updateData)

	if err != nil {
		return errors.NewError(domain.ErrInternal, err)
	}

	if res.MatchedCount == 0 {
		return errors.NewError(domain.ErrNotFound, stderrors.New("user not found"))
	}

	return nil

}

// buildUpdateDoc uses reflection to create a BSON document with only non-nil pointer fields
func buildUpdateDoc(update interface{}) bson.M {
	updateDoc := bson.M{}
	v := reflect.ValueOf(update)

	//Derefrence if its a pointer
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		//Only pointer fileds that are not nil
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			bsonTag := fieldType.Tag.Get("bson")
			if bsonTag != "" && bsonTag != "-" {
				//For nested structs set, the entire struct
				updateDoc[bsonTag] = field.Elem().Interface()
			}
		}
	}
	return updateDoc
}
