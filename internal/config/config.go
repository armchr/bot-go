package config

import (
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

type SourceConfig struct {
	Repositories []Repository `yaml:"repositories"`
}

type Repository struct {
	Name               string `yaml:"name"`
	Path               string `yaml:"path"`
	Test               string `yaml:"test,omitempty"`
	Language           string `yaml:"language"`
	Disabled           bool   `yaml:"disabled,omitempty"`
	SkipOtherLanguages bool   `yaml:"skip_other_languages,omitempty"`
}

type App struct {
	Port           int    `yaml:"port"`
	CodeGraph      bool   `yaml:"codegraph"`
	Gopls          string `yaml:"gopls"`
	Python         string `yaml:"python"`
	WorkDir        string `yaml:"workdir,omitempty"`
	GCThreshold    int64  `yaml:"gc_threshold,omitempty"`
	NumFileThreads int    `yaml:"num_file_threads,omitempty"`
}

type McpConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type Neo4jConfig struct {
	URI      string `yaml:"uri"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type KuzuConfig struct {
	Path string `yaml:"path"`
}

type QdrantConfig struct {
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	APIKey string `yaml:"apikey"`
}

type OllamaConfig struct {
	URL       string `yaml:"url"`
	APIKey    string `yaml:"apikey"`
	Model     string `yaml:"model"`
	Dimension int    `yaml:"dimension"`
}

type ChunkingConfig struct {
	MinConditionalLines int `yaml:"min_conditional_lines"`
	MinLoopLines        int `yaml:"min_loop_lines"`
}

type BloomFilterConfig struct {
	Enabled           bool    `yaml:"enabled"`
	StorageDir        string  `yaml:"storage_dir"`
	ExpectedItems     uint    `yaml:"expected_items"`
	FalsePositiveRate float64 `yaml:"false_positive_rate"`
}

type IndexBuildingConfig struct {
	EnableCodeGraph  bool `yaml:"enable_code_graph"`
	EnableEmbeddings bool `yaml:"enable_embeddings"`
	EnableNgram      bool `yaml:"enable_ngram"`
}

type MySQLConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

type CodeGraphConfig struct {
	EnableBatchWrites bool `yaml:"enable_batch_writes"`
	BatchSize         int  `yaml:"batch_size"` // Number of nodes/relations to batch before writing
}

func (c *McpConfig) GetAddress() string {
	//return fmt.Sprintf("%s:%d", c.Host, c.Port) //, c.Path)
	return fmt.Sprintf(":%d", c.Port) //, c.Path)
}

type Config struct {
	Source        SourceConfig        `yaml:"source"`
	Mcp           McpConfig           `yaml:"mcp"`
	Neo4j         Neo4jConfig         `yaml:"neo4j"`
	Kuzu          KuzuConfig          `yaml:"kuzu"`
	Qdrant        QdrantConfig        `yaml:"qdrant"`
	Chunking      ChunkingConfig      `yaml:"chunking"`
	Ollama        OllamaConfig        `yaml:"ollama"`
	BloomFilter   BloomFilterConfig   `yaml:"bloom_filter"`
	IndexBuilding IndexBuildingConfig `yaml:"index_building"`
	MySQL         MySQLConfig         `yaml:"mysql"`
	CodeGraph     CodeGraphConfig     `yaml:"code_graph"`
	App           App                 `yaml:"app"`
}

func LoadConfig(appConfigPath string, sourceConfigPath string) (*Config, error) {
	if _, err := os.Stat(appConfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("app config file does not exist: %s", appConfigPath)
	}
	if _, err := os.Stat(sourceConfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("source config file does not exist: %s", sourceConfigPath)
	}

	dataApp, err := ioutil.ReadFile(appConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read app config file: %w", err)
	}

	dataSource, err := ioutil.ReadFile(sourceConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source config file: %w", err)
	}

	var configApp Config
	if err := yaml.Unmarshal(dataApp, &configApp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal app config: %w", err)
	}

	var configSource Config
	if err := yaml.Unmarshal(dataSource, &configSource); err != nil {
		return nil, fmt.Errorf("failed to unmarshal source config: %w", err)
	}

	// Merge SourceConfig into configApp
	configApp.Source = configSource.Source

	// Validate repository configurations
	if err := validateRepositories(&configApp); err != nil {
		return nil, fmt.Errorf("invalid repository configuration: %w", err)
	}

	if configSource.Mcp.Host != "" {
		configApp.Mcp = configSource.Mcp
	}

	if configSource.Neo4j.URI != "" {
		configApp.Neo4j = configSource.Neo4j
	}

	if configSource.Kuzu.Path != "" {
		configApp.Kuzu = configSource.Kuzu
	}

	if configSource.Qdrant.Host != "" {
		configApp.Qdrant = configSource.Qdrant
	}

	if configSource.Ollama.URL != "" {
		configApp.Ollama = configSource.Ollama
	}

	return &configApp, nil
}

func (c *Config) GetRepository(name string) (*Repository, error) {
	for _, repo := range c.Source.Repositories {
		if repo.Name == name {
			return &repo, nil
		}
	}
	return nil, fmt.Errorf("repository not found: %s", name)
}

// validateRepositories validates repository configurations
func validateRepositories(config *Config) error {
	for _, repo := range config.Source.Repositories {
		// If skip_other_languages is true, language must be specified
		if repo.SkipOtherLanguages && repo.Language == "" {
			return fmt.Errorf("repository '%s': skip_other_languages is true but language is not specified", repo.Name)
		}
	}
	return nil
}
