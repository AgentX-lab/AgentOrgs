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

// EnsureMemberWorkspace writes default workspace files only when SOUL.md is absent.
func (s *Store) EnsureMemberWorkspace(ctx context.Context, namespace, memberName, displayName string) error {
	soulKey := workspace.MemberPrefix(namespace, memberName) + "SOUL.md"
	_, err := s.client.StatObject(ctx, s.bucket, soulKey, minio.StatObjectOptions{})
	if err == nil {
		return nil
	}

	return fs.WalkDir(workspace.Templates, "templates", func(p string, d fs.DirEntry, walkErr error) error {
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
		content := string(data)
		if path.Base(rel) == "SOUL.md" {
			content = workspace.Render(content, displayName)
		}
		key := workspace.MemberPrefix(namespace, memberName) + rel
		contentType := "text/plain"
		if strings.HasSuffix(rel, ".json") {
			contentType = "application/json"
		}
		_, putErr := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader([]byte(content)), int64(len(content)), minio.PutObjectOptions{
			ContentType: contentType,
		})
		return putErr
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
