package slot

// Slot returns the Redis Cluster hash slot for key. The CRC16 implementation
// follows Redis' XMODEM variant and is compatible with Redis 5.0+.
func Slot(key string) int {
	return int(CRC16([]byte(HashTag(key))) % 16384)
}

func HashTag(key string) string {
	for i := 0; i < len(key); i++ {
		if key[i] != '{' {
			continue
		}
		for j := i + 1; j < len(key); j++ {
			if key[j] == '}' {
				if j > i+1 {
					return key[i+1 : j]
				}
				// Redis only considers the first '{' and its following '}'.
				// An empty pair disables hash-tag extraction for the key.
				return key
			}
		}
		return key
	}
	return key
}

func CRC16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
