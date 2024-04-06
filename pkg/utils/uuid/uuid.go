package uuid

import (
	"encoding/hex"

	"github.com/gofrs/uuid"
)

func GenUUID4() string {
	return hex.EncodeToString(uuid.Must(uuid.NewV4()).Bytes())
}
