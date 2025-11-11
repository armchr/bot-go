func NewPythonLanguageServerClient(config *config.Config, rootPath string, logger *zap.Logger) (*PythonLanguageServerClient, error) {
	logger.Info("Creating new Python language server client")
	base, err := NewBaseClient(config.App.Python, logger)
	if err != nil {
		return nil, err
	}

	t := &PythonLanguageServerClient{BaseClient: base, rootPath: rootPath, logger: logger}
	t.client = t
	return t, nil
}
