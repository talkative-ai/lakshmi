package helpers

type BSliceIndex struct {
	Index  int
	Bslice []byte
}

// RedisBytes is used to communicate values to be written to Redis
type RedisBytes struct {
	Key   string
	Bytes []byte
}
