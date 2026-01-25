package utils

import "go.mongodb.org/mongo-driver/bson/primitive"

func HexToObjectID(str string) (primitive.ObjectID, error) {
	objId, err := primitive.ObjectIDFromHex(str)
	return objId, err
}
