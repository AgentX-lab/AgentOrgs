package matrix

import (
	"fmt"
	"strings"

	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/config"
	"gopkg.in/yaml.v3"
)

// AppServiceRegistration is the YAML Tuwunel expects for appservice register.
type AppServiceRegistration struct {
	ID              string               `yaml:"id"`
	URL             string               `yaml:"url"`
	ASToken         string               `yaml:"as_token"`
	HSToken         string               `yaml:"hs_token"`
	SenderLocalpart string               `yaml:"sender_localpart"`
	RateLimited     bool                 `yaml:"rate_limited"`
	Namespaces      AppServiceNamespaces `yaml:"namespaces"`
}

type AppServiceNamespaces struct {
	Users   []AppServiceNamespace `yaml:"users"`
	Aliases []AppServiceNamespace `yaml:"aliases"`
	Rooms   []AppServiceNamespace `yaml:"rooms"`
}

type AppServiceNamespace struct {
	Exclusive bool   `yaml:"exclusive"`
	Regex     string `yaml:"regex"`
}

// RegistrationYAML builds the AppService registration document.
func RegistrationYAML(cfg config.Config) ([]byte, error) {
	push := strings.TrimRight(cfg.MatrixAppServicePushURL, "/")
	reg := AppServiceRegistration{
		ID:              cfg.MatrixAppServiceID,
		URL:             push,
		ASToken:         cfg.MatrixAppServiceASToken,
		HSToken:         cfg.MatrixAppServiceHSToken,
		SenderLocalpart: cfg.MatrixAppServiceSenderLocalpart,
		RateLimited:     false,
		Namespaces: AppServiceNamespaces{
			Users: []AppServiceNamespace{
				{Exclusive: true, Regex: fmt.Sprintf("@.*:%s", cfg.MatrixDomain)},
			},
			Aliases: []AppServiceNamespace{
				{Exclusive: false, Regex: fmt.Sprintf("#agentorgs-.*:%s", cfg.MatrixDomain)},
			},
			Rooms: []AppServiceNamespace{},
		},
	}
	return yaml.Marshal(reg)
}
