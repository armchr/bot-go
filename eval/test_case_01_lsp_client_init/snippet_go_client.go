func NewGoLanguageServerClient(config *config.Config, rootPath string, logger *zap.Logger) (*GoLanguageServerClient, error) {
	logger.Info("Creating new Go language server client")
	base, err := NewBaseClient(config.App.Gopls, logger)
	if err != nil {
		return nil, err
	}

	t := &GoLanguageServerClient{BaseClient: base, rootPath: rootPath, logger: logger}
	t.client = t
	return t, nil
}
