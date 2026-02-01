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

// Ensure MongoUserRepository implements ports.UserRepository
var _ ports.UserRepository = (*MongoUserRepository)(nil)

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

// Delete removes a user by ID permanently
func (r *MongoUserRepository) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.NewError(domain.ErrInvalidInput, err)
	}

	filter := bson.M{"_id": oid}
	res, err := r.coll.DeleteOne(ctx, filter)
	if err != nil {
		return errors.NewError(domain.ErrInternal, err)
	}

	if res.DeletedCount == 0 {
		return errors.NewError(domain.ErrNotFound, stderrors.New("user not found"))
	}

	return nil
}

// ListUsers returns a paginated list of users with optional filtering
func (r *MongoUserRepository) ListUsers(ctx context.Context, filter ports.UserFilter) ([]domain.User, error) {
	// Build filter document
	filterDoc := bson.M{}
	if filter.SubscriptionTier != nil {
		filterDoc["subscription.tier"] = *filter.SubscriptionTier
	}
	if filter.Email != nil {
		filterDoc["email"] = bson.M{"$regex": *filter.Email, "$options": "i"}
	}

	// Set default pagination values
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100 // Max limit to prevent abuse
	}

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// Set default sort
	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := filter.SortOrder
	if sortOrder == 0 {
		sortOrder = -1 // Default to descending (newest first)
	}

	opts := options.Find().
		SetLimit(limit).
		SetSkip(offset).
		SetSort(bson.D{{Key: sortBy, Value: sortOrder}})

	cursor, err := r.coll.Find(ctx, filterDoc, opts)
	if err != nil {
		return nil, errors.NewError(domain.ErrInternal, err)
	}
	defer cursor.Close(ctx)

	var users []domain.User
	if err = cursor.All(ctx, &users); err != nil {
		return nil, errors.NewError(domain.ErrInternal, err)
	}

	return users, nil
}

// Count returns the total number of users matching the filter
func (r *MongoUserRepository) Count(ctx context.Context, filter ports.UserFilter) (int64, error) {
	// Build filter document (same as ListUsers but without pagination)
	filterDoc := bson.M{}
	if filter.SubscriptionTier != nil {
		filterDoc["subscription.tier"] = *filter.SubscriptionTier
	}
	if filter.Email != nil {
		filterDoc["email"] = bson.M{"$regex": *filter.Email, "$options": "i"}
	}

	count, err := r.coll.CountDocuments(ctx, filterDoc)
	if err != nil {
		return 0, errors.NewError(domain.ErrInternal, err)
	}

	return count, nil
}

// ExistsByEmail checks if a user with the given email exists
func (r *MongoUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	filter := bson.M{"email": email}
	count, err := r.coll.CountDocuments(ctx, filter, options.Count().SetLimit(1))
	if err != nil {
		return false, errors.NewError(domain.ErrInternal, err)
	}
	return count > 0, nil
}

// FindBySubscriptionTier returns all users with the specified subscription tier
func (r *MongoUserRepository) FindBySubscriptionTier(ctx context.Context, tier domain.SubscriptionTier) ([]domain.User, error) {
	filter := bson.M{"subscription.tier": tier}
	opts := options.Find().SetSort(bson.D{
		{Key: "created_at", Value: -1},
	})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, errors.NewError(domain.ErrInternal, err)
	}
	defer cursor.Close(ctx)

	var users []domain.User
	if err = cursor.All(ctx, &users); err != nil {
		return nil, errors.NewError(domain.ErrInternal, err)
	}

	return users, nil
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
