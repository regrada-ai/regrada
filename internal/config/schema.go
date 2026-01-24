package config

type ProjectConfig struct {
	Version   int             `yaml:"version"`
	Project   ProjectMeta     `yaml:"project,omitempty"`
	Cases     CasesConfig     `yaml:"cases,omitempty"`
	Capture   CaptureConfig   `yaml:"capture,omitempty"`
	Providers ProvidersConfig `yaml:"providers,omitempty"`
	Baseline  BaselineConfig  `yaml:"baseline,omitempty"`
	Policies  []Policy        `yaml:"policies,omitempty"`
	Report    ReportConfig    `yaml:"report,omitempty"`
	CI        CIConfig        `yaml:"ci,omitempty"`
	Record    RecordConfig    `yaml:"record,omitempty"`
	Backend   BackendConfig   `yaml:"backend,omitempty"`
}

type ProjectMeta struct {
	Name string   `yaml:"name,omitempty"`
	Root string   `yaml:"root,omitempty"`
	Tags []string `yaml:"tags,omitempty"`
}

type CasesConfig struct {
	Roots    []string     `yaml:"roots,omitempty"`
	Include  []string     `yaml:"include,omitempty"`
	Exclude  []string     `yaml:"exclude,omitempty"`
	Defaults CaseDefaults `yaml:"defaults,omitempty"`
}

type CaseDefaults struct {
	Runs        int            `yaml:"runs,omitempty"`
	TimeoutMS   int            `yaml:"timeout_ms,omitempty"`
	Concurrency int            `yaml:"concurrency,omitempty"`
	Sampling    SamplingConfig `yaml:"sampling,omitempty"`
}

type SamplingConfig struct {
	Temperature     *float64 `yaml:"temperature,omitempty"`
	TopP            *float64 `yaml:"top_p,omitempty"`
	MaxOutputTokens *int     `yaml:"max_output_tokens,omitempty"`
}

type CaptureConfig struct {
	Enabled *bool        `yaml:"enabled,omitempty"`
	Mode    string       `yaml:"mode,omitempty"`
	Proxy   ProxyConfig  `yaml:"proxy,omitempty"`
	Redact  RedactConfig `yaml:"redact,omitempty"`
}

type ProxyConfig struct {
	Listen     string         `yaml:"listen,omitempty"`
	Mode       string         `yaml:"mode,omitempty"` // "forward" or "reverse"
	CAPath     string         `yaml:"ca_path,omitempty"`
	AllowHosts []string       `yaml:"allow_hosts,omitempty"`
	Debug      bool           `yaml:"debug,omitempty"`
	Upstream   UpstreamConfig `yaml:"upstream,omitempty"` // For reverse proxy mode
}

type UpstreamConfig struct {
	OpenAIBaseURL      string `yaml:"openai_base_url,omitempty"`
	AnthropicBaseURL   string `yaml:"anthropic_base_url,omitempty"`
	AzureOpenAIBaseURL string `yaml:"azure_openai_base_url,omitempty"`
	BedrockBaseURL     string `yaml:"bedrock_base_url,omitempty"`
}

type RedactConfig struct {
	Enabled  *bool           `yaml:"enabled,omitempty"`
	Presets  []string        `yaml:"presets,omitempty"`
	Patterns []RedactPattern `yaml:"patterns,omitempty"`
}

type RedactPattern struct {
	Name        string `yaml:"name"`
	Regex       string `yaml:"regex"`
	ReplaceWith string `yaml:"replace_with"`
}

type ProvidersConfig struct {
	Default     string            `yaml:"default,omitempty"`
	OpenAI      OpenAIConfig      `yaml:"openai,omitempty"`
	Anthropic   AnthropicConfig   `yaml:"anthropic,omitempty"`
	AzureOpenAI AzureOpenAIConfig `yaml:"azure_openai,omitempty"`
	Bedrock     BedrockConfig     `yaml:"bedrock,omitempty"`
}

type OpenAIConfig struct {
	APIKeyEnv  string `yaml:"api_key_env,omitempty"`
	APIKey     string `yaml:"api_key,omitempty"`
	BaseURLEnv string `yaml:"base_url_env,omitempty"`
	BaseURL    string `yaml:"base_url,omitempty"`
	Model      string `yaml:"model,omitempty"`
}

type AnthropicConfig struct {
	APIKeyEnv  string `yaml:"api_key_env,omitempty"`
	APIKey     string `yaml:"api_key,omitempty"`
	BaseURLEnv string `yaml:"base_url_env,omitempty"`
	BaseURL    string `yaml:"base_url,omitempty"`
	Model      string `yaml:"model,omitempty"`
}

type AzureOpenAIConfig struct {
	APIKeyEnv   string `yaml:"api_key_env,omitempty"`
	APIKey      string `yaml:"api_key,omitempty"`
	EndpointEnv string `yaml:"endpoint_env,omitempty"`
	Endpoint    string `yaml:"endpoint,omitempty"`
	APIVersion  string `yaml:"api_version,omitempty"`
	Deployment  string `yaml:"deployment,omitempty"`
}

type BedrockConfig struct {
	RegionEnv    string `yaml:"region_env,omitempty"`
	Region       string `yaml:"region,omitempty"`
	AccessKeyEnv string `yaml:"access_key_env,omitempty"`
	AccessKey    string `yaml:"access_key,omitempty"`
	SecretKeyEnv string `yaml:"secret_key_env,omitempty"`
	SecretKey    string `yaml:"secret_key,omitempty"`
	ModelID      string `yaml:"model_id,omitempty"`
}

type BaselineConfig struct {
	Mode  string              `yaml:"mode,omitempty"`
	Git   BaselineGitConfig   `yaml:"git,omitempty"`
	Local BaselineLocalConfig `yaml:"local,omitempty"`
}

type BaselineGitConfig struct {
	Ref         string `yaml:"ref,omitempty"`
	SnapshotDir string `yaml:"snapshot_dir,omitempty"`
}

type BaselineLocalConfig struct {
	SnapshotDir string `yaml:"snapshot_dir,omitempty"`
}

type Policy struct {
	ID          string       `yaml:"id"`
	Description string       `yaml:"description,omitempty"`
	Severity    string       `yaml:"severity,omitempty"`
	Scope       *PolicyScope `yaml:"scope,omitempty"`
	Check       PolicyCheck  `yaml:"check"`
}

type PolicyScope struct {
	Tags      []string `yaml:"tags,omitempty"`
	IDs       []string `yaml:"ids,omitempty"`
	Providers []string `yaml:"providers,omitempty"`
}

type PolicyCheck struct {
	Type         string            `yaml:"type"`
	Extractor    string            `yaml:"extractor,omitempty"`
	MinPassRate  *float64          `yaml:"min_pass_rate,omitempty"`
	Schema       string            `yaml:"schema,omitempty"`
	Phrases      []string          `yaml:"phrases,omitempty"`
	MaxIncidents *int              `yaml:"max_incidents,omitempty"`
	Detector     string            `yaml:"detector,omitempty"`
	Metric       string            `yaml:"metric,omitempty"`
	MaxP95       *float64          `yaml:"max_p95,omitempty"`
	Max          *float64          `yaml:"max,omitempty"`
	MaxDelta     *float64          `yaml:"max_delta,omitempty"`
	LatencyP95   *LatencyThreshold `yaml:"p95_ms,omitempty"`
}

type LatencyThreshold struct {
	Max      *int `yaml:"max,omitempty"`
	MaxDelta *int `yaml:"max_delta,omitempty"`
}

type ReportConfig struct {
	Format         []string       `yaml:"format,omitempty"`
	Markdown       MarkdownConfig `yaml:"markdown,omitempty"`
	JUnit          JUnitConfig    `yaml:"junit,omitempty"`
	StoreArtifacts *bool          `yaml:"store_artifacts,omitempty"`
}

type MarkdownConfig struct {
	Path string `yaml:"path,omitempty"`
}

type JUnitConfig struct {
	Path string `yaml:"path,omitempty"`
}

type CIConfig struct {
	FailOn      []FailOnSeverity `yaml:"fail_on,omitempty"`
	CommentOnPR *bool            `yaml:"comment_on_pr,omitempty"`
	Paths       *PathFilter      `yaml:"paths,omitempty"`
}

type FailOnSeverity struct {
	Severity string `yaml:"severity"`
}

type PathFilter struct {
	Include []string `yaml:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

type RecordConfig struct {
	SessionDir string       `yaml:"session_dir,omitempty"`
	TracesDir  string       `yaml:"traces_dir,omitempty"`
	Accept     AcceptConfig `yaml:"accept,omitempty"`
}

type AcceptConfig struct {
	OutputDir    string          `yaml:"output_dir,omitempty"`
	DefaultTags  []string        `yaml:"default_tags,omitempty"`
	InferAsserts *bool           `yaml:"infer_asserts,omitempty"`
	Normalize    NormalizeConfig `yaml:"normalize,omitempty"`
}

type NormalizeConfig struct {
	TrimWhitespace     *bool `yaml:"trim_whitespace,omitempty"`
	DropVolatileFields *bool `yaml:"drop_volatile_fields,omitempty"`
}

type BackendConfig struct {
	Enabled   *bool        `yaml:"enabled,omitempty"`
	APIKeyEnv string       `yaml:"api_key_env,omitempty"`
	ProjectID string       `yaml:"project_id,omitempty"`
	Upload    UploadConfig `yaml:"upload,omitempty"`
}

type UploadConfig struct {
	Traces      *bool `yaml:"traces,omitempty"`
	TestResults *bool `yaml:"test_results,omitempty"`
	Async       *bool `yaml:"async,omitempty"`
	BatchSize   int   `yaml:"batch_size,omitempty"`
}
