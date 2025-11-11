// Add adds data to the bloom filter for a repository
func (bfm *BloomFilterManager) Add(repoName string, data string) error {
	filter, err := bfm.GetOrCreateFilter(repoName)
	if err != nil {
		return err
	}

	filter.AddString(data)
	return nil
}

// Test checks if data might exist in the bloom filter for a repository
// Returns true if data might exist (or false positive), false if definitely doesn't exist
func (bfm *BloomFilterManager) Test(repoName string, data string) (bool, error) {
	filter, err := bfm.GetOrCreateFilter(repoName)
	if err != nil {
		return false, err
	}

	return filter.TestString(data), nil
}
