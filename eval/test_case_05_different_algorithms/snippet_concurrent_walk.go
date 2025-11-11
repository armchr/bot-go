func WalkDirTree(root string, walkFn WalkFunc, skipPath SkipFunc, logger *zap.Logger, gcThreshold int64, numThreads int) error {
	processedCount := int64(0)
	// Create channels for work distribution
	workQueue := make(chan walkItem, 2)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Start 2 worker goroutines
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range workQueue {
				// Increment processed count
				mu.Lock()
				processedCount++
				mu.Unlock()

				// Trigger GC if needed
				if gcThreshold > 0 && processedCount%gcThreshold == 0 {
					logger.Info("WalkDirTree - Triggering GC after processing files",
						zap.Int64("files_processed", processedCount))
					runtime.GC()
				}

				// Call the walk function
				err := walkFn(item.path, nil)
				if err != nil {
					logger.Error("WalkDirTree - Failed to process file", zap.String("path", item.path), zap.Error(err))
					if err != filepath.SkipDir {
						// Continue processing other files even if one fails
					}
				}
			}
		}()
	}

	// Walk the directory tree and send items to workers
	err := walk(root, workQueue, skipPath, &processedCount, gcThreshold, logger)
	close(workQueue)

	// Wait for all workers to finish
	wg.Wait()

	return err
}
