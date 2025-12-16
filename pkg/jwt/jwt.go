package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var secretKey = []byte("todo-add-secrect-key")

func CreateToken(email, _id string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodES256,
		jwt.MapClaims{
			"email": email,
			"_id":   _id,
			"exp":   time.Now().Add(time.Hour * 24).Unix(),
		})

	tokenStr, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}

	return tokenStr, nil
}

func VerifyToken(tokenString string) error {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})

	if err != nil {
		return err
	}

	if !token.Valid {
		return fmt.Errorf("invalid token")
	}

	return nil
}
