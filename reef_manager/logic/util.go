package logic

import "encoding/hex"

func IdToString(id NodeId) string {
	return hex.EncodeToString(id[0:])
}
