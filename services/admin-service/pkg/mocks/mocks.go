package mocks

import (
	"math/rand"
	"time"

	"github.com/jayant-dispral/brand-threat-be/shared/domain"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func GenerateMockBrandMoniterEvent() *domain.BrandMoniterEvent {
	
	return  &domain.BrandMoniterEvent{
		ProjectID: randStringBytes(10),
		KeyWord: randStringBytes(4),
		RequestedBy: randStringBytes(8),
		TimeStamp: time.Now(),
	}
}

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range n {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}