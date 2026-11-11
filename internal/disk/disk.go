package disk

func Free(path string) (int64, error) {
	return free(path)
}
