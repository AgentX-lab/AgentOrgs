package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds controller runtime settings.
type Config struct {
	HTTPAddr        string
	MetricsBindAddr string
	Namespace       string

	OpenClawAgentImage string
	HermesAgentImage   string

	LLMAPIKey    string
	LLMBaseURL   string
	DefaultModel string

	// Matrix homeserver connection.
	MatrixHomeserver  string
	MatrixDomain      string
	MatrixAccessToken string // optional sender token; AppService AS token is preferred

	// Matrix AppService (homeserver pushes room events to the controller).
	MatrixAppServiceEnabled        bool
	MatrixAppServiceID             string
	MatrixAppServiceASToken        string
	MatrixAppServiceHSToken        string
	MatrixAppServiceSenderLocalpart string
	MatrixAppServicePushURL        string

	// MatrixSetupEnabled (env AGENTORGS_MATRIX_BOOTSTRAP) creates Matrix users/rooms for Members and Groups.
	MatrixBootstrapEnabled bool
	MatrixAdminUser        string
	MatrixAdminPassword    string

	MinIOEndpoint        string
	MinIOAccessKey       string
	MinIOSecretKey       string
	MinIOBucket          string
	MinIOUseSSL          bool
	WorkspaceSyncSeconds int
}

func Load() Config {
	return Config{
		HTTPAddr:        env("AGENTORGS_HTTP_ADDR", ":8090"),
		MetricsBindAddr: env("AGENTORGS_METRICS_ADDR", ":8080"),
		Namespace:       env("AGENTORGS_NAMESPACE", "agentorgs"),

		OpenClawAgentImage: env("AGENTORGS_OPENCLAW_AGENT_IMAGE", "agentorgs/agent-openclaw:local"),
		HermesAgentImage:   env("AGENTORGS_HERMES_AGENT_IMAGE", "agentorgs/agent-hermes:local"),

		LLMAPIKey:    os.Getenv("AGENTORGS_LLM_API_KEY"),
		LLMBaseURL:   env("AGENTORGS_LLM_BASE_URL", "https://api.openai.com/v1"),
		DefaultModel: env("AGENTORGS_DEFAULT_MODEL", "gpt-4o-mini"),

		MatrixHomeserver:  env("AGENTORGS_MATRIX_HOMESERVER", "http://tuwunel:6167"),
		MatrixDomain:      env("AGENTORGS_MATRIX_DOMAIN", "matrix-local.agentorgs.io"),
		MatrixAccessToken: os.Getenv("AGENTORGS_MATRIX_ACCESS_TOKEN"),

		MatrixAppServiceEnabled:         envBool("AGENTORGS_MATRIX_APPSERVICE_ENABLED", true),
		MatrixAppServiceID:              env("AGENTORGS_MATRIX_APPSERVICE_ID", "agentorgs-controller"),
		MatrixAppServiceASToken:         os.Getenv("AGENTORGS_MATRIX_APPSERVICE_AS_TOKEN"),
		MatrixAppServiceHSToken:         os.Getenv("AGENTORGS_MATRIX_APPSERVICE_HS_TOKEN"),
		MatrixAppServiceSenderLocalpart: env("AGENTORGS_MATRIX_APPSERVICE_SENDER", "agentorgs-bot"),
		MatrixAppServicePushURL:         env("AGENTORGS_MATRIX_APPSERVICE_PUSH_URL", "http://agentorgs-controller:8090"),

		MatrixBootstrapEnabled: envBool("AGENTORGS_MATRIX_BOOTSTRAP", true),
		MatrixAdminUser:        env("AGENTORGS_MATRIX_ADMIN_USER", "admin"),
		MatrixAdminPassword:    env("AGENTORGS_MATRIX_ADMIN_PASSWORD", "admin"),

		MinIOEndpoint:        env("AGENTORGS_MINIO_ENDPOINT", "minio:9000"),
		MinIOAccessKey:       env("AGENTORGS_MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:       env("AGENTORGS_MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:          env("AGENTORGS_MINIO_BUCKET", "agentorgs"),
		MinIOUseSSL:          envBool("AGENTORGS_MINIO_USE_SSL", false),
		WorkspaceSyncSeconds: envInt("AGENTORGS_SYNC_INTERVAL_SECONDS", 30),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// Validate checks required settings. Matrix AppService tokens must come from
// env/Secret (Helm or local install); the controller never generates them.
func (c Config) Validate() error {
	if !c.MatrixAppServiceEnabled {
		return nil
	}
	if c.MatrixAppServiceASToken == "" || c.MatrixAppServiceHSToken == "" {
		return fmt.Errorf("AGENTORGS_MATRIX_APPSERVICE_AS_TOKEN and AGENTORGS_MATRIX_APPSERVICE_HS_TOKEN are required when AppService is enabled (set them via Secret/Helm)")
	}
	return nil
}
