package logic

import "encoding/hex"

func IDToString(id NodeID) string {
	return hex.EncodeToString(id[0:])
}
