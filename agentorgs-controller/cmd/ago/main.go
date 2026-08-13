package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "ago",
		Short: "AgentOrgs command-line tool",
	}
	root.AddCommand(collaborateCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func collaborateCmd() *cobra.Command {
	var (
		apiURL            string
		namespace         string
		collaborationName string
		from string
		to   string
		text string
	)
	cmd := &cobra.Command{
		Use:   "collaborate",
		Short: "Start a collaboration",
		RunE: func(_ *cobra.Command, _ []string) error {
			body := map[string]interface{}{
				"from": from,
				"to": []protocol.ObjectTarget{
					{Kind: "Member", Name: to},
				},
				"payload": map[string]interface{}{"text": text},
			}
			data, err := json.Marshal(body)
			if err != nil {
				return err
			}
			url := fmt.Sprintf("%s/api/v1/collaborations/%s/%s/runs", apiURL, namespace, collaborationName)
			resp, err := http.Post(url, "application/json", bytes.NewReader(data))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 300 {
				return fmt.Errorf("request failed: %s", resp.Status)
			}
			var run protocol.CollaborationRun
			if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
				return err
			}
			fmt.Printf("run started: %s status=%s\n", run.RunID, run.Status)
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api", env("AGENTORGS_API_URL", "http://127.0.0.1:8090"), "AgentOrgs API URL")
	cmd.Flags().StringVar(&namespace, "namespace", "agentorgs", "Kubernetes namespace")
	cmd.Flags().StringVar(&collaborationName, "collaboration", "backend-work", "Collaboration name")
	cmd.Flags().StringVar(&from, "from", "product-owner", "Who starts this collaboration")
	cmd.Flags().StringVar(&to, "to", "developer", "Who should receive it")
	cmd.Flags().StringVar(&text, "text", "implement login API", "Request text")
	return cmd
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
