func NewTypeScriptLanguageServerClient(rootPath string, logger *zap.Logger) (*TypeScriptLanguageServerClient, error) {
	logger.Info("Creating new TypeScript language server client")
	base, err := NewBaseClient("typescript-language-server", logger, "--stdio")
	if err != nil {
		return nil, err
	}

	t := &TypeScriptLanguageServerClient{BaseClient: base, rootPath: rootPath, logger: logger}
	t.client = t
	return t, nil
}
