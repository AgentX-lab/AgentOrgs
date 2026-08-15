package minio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/config"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/workspace"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const providerName = "minio"

// Store persists collaboration runs/events and Member workspaces in MinIO.
type Store struct {
	client *minio.Client
	bucket string
}

func NewStore(cfg config.Config) (*Store, error) {
	client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOUseSSL,
	})
	if err != nil {
		return nil, err
	}
	store := &Store{client: client, bucket: cfg.MinIOBucket}
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.MinIOBucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.MinIOBucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (s *Store) Name() string { return providerName }

func (s *Store) WriteRun(ctx context.Context, run protocol.CollaborationRun) error {
	return s.putJSON(ctx, runKey(run.Namespace, run.RunID), run)
}

func (s *Store) ReadRun(ctx context.Context, namespace, runID string) (protocol.CollaborationRun, error) {
	var run protocol.CollaborationRun
	if err := s.getJSON(ctx, runKey(namespace, runID), &run); err != nil {
		return protocol.CollaborationRun{}, err
	}
	return run, nil
}

func (s *Store) WriteEvent(ctx context.Context, event protocol.CollaborationEvent) error {
	return s.putJSON(ctx, eventKey(event.Namespace, event.RunID, event.EventID), event)
}

func (s *Store) ListEvents(ctx context.Context, namespace, runID string) ([]protocol.CollaborationEvent, error) {
	prefix := fmt.Sprintf("%s/runs/%s/events/", namespace, runID)
	ch := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
	var events []protocol.CollaborationEvent
	for obj := range ch {
		if obj.Err != nil {
			return nil, obj.Err
		}
		var event protocol.CollaborationEvent
		if err := s.getJSON(ctx, obj.Key, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

// EnsureMemberWorkspace seeds base persona/config when SOUL.md is absent, and
// seeds each resolved skill only when that skill's SKILL.md is absent.
func (s *Store) EnsureMemberWorkspace(ctx context.Context, namespace, memberName, displayName, skillPack string, extraSkills []string) error {
	skills, err := workspace.ResolveSkills(skillPack, extraSkills)
	if err != nil {
		return err
	}

	soulKey := workspace.MemberPrefix(namespace, memberName) + "SOUL.md"
	if _, err := s.client.StatObject(ctx, s.bucket, soulKey, minio.StatObjectOptions{}); err != nil {
		for _, name := range []string{"SOUL.md", "AGENTS.md", "openclaw.json"} {
			if putErr := s.putTemplateFile(ctx, namespace, memberName, name, displayName); putErr != nil {
				return putErr
			}
		}
	}

	for _, skill := range skills {
		marker := "skills/" + skill + "/SKILL.md"
		if _, err := s.GetWorkspaceFile(ctx, namespace, memberName, marker); err == nil {
			continue
		}
		if putErr := s.putSkillDir(ctx, namespace, memberName, skill); putErr != nil {
			return putErr
		}
	}
	return nil
}

func (s *Store) putTemplateFile(ctx context.Context, namespace, memberName, rel, displayName string) error {
	data, err := workspace.Templates.ReadFile("templates/" + rel)
	if err != nil {
		return err
	}
	content := string(data)
	if path.Base(rel) == "SOUL.md" {
		content = workspace.Render(content, displayName)
	}
	return s.PutWorkspaceFile(ctx, namespace, memberName, rel, []byte(content))
}

func (s *Store) putSkillDir(ctx context.Context, namespace, memberName, skill string) error {
	root := "templates/skills/" + skill
	return fs.WalkDir(workspace.Templates, root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		data, readErr := workspace.Templates.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		rel := strings.TrimPrefix(p, "templates/")
		return s.PutWorkspaceFile(ctx, namespace, memberName, rel, data)
	})
}

// GetWorkspaceFile reads one file from a Member workspace.
func (s *Store) GetWorkspaceFile(ctx context.Context, namespace, memberName, relativePath string) ([]byte, error) {
	key := workspace.MemberPrefix(namespace, memberName) + strings.TrimPrefix(relativePath, "/")
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(obj); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// PutWorkspaceFile writes one file into a Member workspace.
func (s *Store) PutWorkspaceFile(ctx context.Context, namespace, memberName, relativePath string, data []byte) error {
	key := workspace.MemberPrefix(namespace, memberName) + strings.TrimPrefix(relativePath, "/")
	contentType := "application/octet-stream"
	if strings.HasSuffix(relativePath, ".json") {
		contentType = "application/json"
	} else if strings.HasSuffix(relativePath, ".md") {
		contentType = "text/plain"
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *Store) putJSON(ctx context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "application/json"})
	return err
}

func (s *Store) getJSON(ctx context.Context, key string, value interface{}) error {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return err
	}
	defer obj.Close()
	return json.NewDecoder(obj).Decode(value)
}

func runKey(namespace, runID string) string {
	return fmt.Sprintf("%s/runs/%s/run.json", namespace, runID)
}

func eventKey(namespace, runID, eventID string) string {
	return fmt.Sprintf("%s/runs/%s/events/%s.json", namespace, runID, eventID)
}
