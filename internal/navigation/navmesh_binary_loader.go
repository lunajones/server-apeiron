package navigation

import (
	"context"
	"fmt"
)

type BinaryLoader struct{}

func NewBinaryLoader() *BinaryLoader {
	return &BinaryLoader{}
}

func (l *BinaryLoader) Load(context.Context, string) (*NavMeshRuntime, error) {
	return nil, fmt.Errorf("binary navmesh loader is reserved for future build pipeline")
}
