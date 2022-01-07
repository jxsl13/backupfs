package mocks_test

import (
	"testing"

	gomock "github.com/golang/mock/gomock"
)

func TestMockFs(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

}
