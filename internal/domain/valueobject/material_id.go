package valueobject

import (
	"github.com/edugo/shared/pkg/types"
)

type MaterialID struct {
	value types.UUID
}

func MaterialIDFromString(s string) (MaterialID, error) {
	uuid, err := types.ParseUUID(s)
	if err != nil {
		return MaterialID{}, err
	}
	return MaterialID{value: uuid}, nil
}

func (m MaterialID) String() string {
	return m.value.String()
}

func (m MaterialID) IsZero() bool {
	return m.value.IsZero()
}
