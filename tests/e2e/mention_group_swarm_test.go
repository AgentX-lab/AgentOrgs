//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// TestMentionGroupSwarm covers flat All-strategy collaboration:
//  1. human @Group → both peer-a and peer-b reply
//  2. peer-a @peer-b → peer-b replies
//  3. peer-b @peer-a → peer-a replies (mutual)
func TestMentionGroupSwarm(t *testing.T) {
	e := loadSwarmEnv()
	cs := kubeClient(t)
	ctx := context.Background()
	roomID := waitMentionGroupSwarmReady(t, e)

	requesterTok, err := secretString(ctx, cs, e.Namespace, "matrix-requester", "accessToken")
	if err != nil {
		t.Fatalf("requester token: %v", err)
	}
	peerATok, err := secretString(ctx, cs, e.Namespace, "matrix-peer-a", "accessToken")
	if err != nil {
		t.Fatalf("peer-a token: %v", err)
	}
	peerBTok, err := secretString(ctx, cs, e.Namespace, "matrix-peer-b", "accessToken")
	if err != nil {
		t.Fatalf("peer-b token: %v", err)
	}

	reqMX := &matrixClient{base: e.MatrixURL, token: requesterTok, http: &http.Client{Timeout: 20 * time.Second}}
	peerAMX := &matrixClient{base: e.MatrixURL, token: peerATok, http: &http.Client{Timeout: 20 * time.Second}}
	peerBMX := &matrixClient{base: e.MatrixURL, token: peerBTok, http: &http.Client{Timeout: 20 * time.Second}}

	before, err := reqMX.recentMessages(ctx, roomID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	peerABefore := senderMessageCount(before, e.PeerAMXID)
	peerBBefore := senderMessageCount(before, e.PeerBMXID)

	if err := reqMX.sendMention(ctx, roomID, "E2E_SWARM_GROUP_WAKE please reply", []string{e.GroupMXID}); err != nil {
		t.Fatalf("human @swarm-team: %v", err)
	}

	if !t.Run("both_peers_after_at_group", func(t *testing.T) {
		waitUntil(t, stepTimeout, "peer-a and peer-b reply after @Group", func(ctx context.Context) (bool, error) {
			events, err := reqMX.recentMessages(ctx, roomID)
			if err != nil {
				return false, err
			}
			aOK := senderMessageCount(events, e.PeerAMXID) > peerABefore &&
				senderHasText(events, e.PeerAMXID, e.ExpectedText)
			bOK := senderMessageCount(events, e.PeerBMXID) > peerBBefore &&
				senderHasText(events, e.PeerBMXID, e.ExpectedText)
			return aOK && bOK, nil
		})
	}) {
		return
	}

	mid, err := reqMX.recentMessages(ctx, roomID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	peerBMid := senderMessageCount(mid, e.PeerBMXID)

	if err := peerAMX.sendMention(ctx, roomID, "E2E_SWARM_PEER_A_TO_B please reply", []string{e.PeerBMXID}); err != nil {
		t.Fatalf("peer-a @peer-b: %v", err)
	}

	if !t.Run("peer_b_after_peer_a_mention", func(t *testing.T) {
		waitUntil(t, stepTimeout, "peer-b reply after peer-a @", func(ctx context.Context) (bool, error) {
			events, err := reqMX.recentMessages(ctx, roomID)
			if err != nil {
				return false, err
			}
			return senderMessageCount(events, e.PeerBMXID) > peerBMid &&
				senderHasText(events, e.PeerBMXID, e.ExpectedText), nil
		})
	}) {
		return
	}

	afterAB, err := reqMX.recentMessages(ctx, roomID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	peerAMid := senderMessageCount(afterAB, e.PeerAMXID)

	if err := peerBMX.sendMention(ctx, roomID, "E2E_SWARM_PEER_B_TO_A please reply", []string{e.PeerAMXID}); err != nil {
		t.Fatalf("peer-b @peer-a: %v", err)
	}

	if !t.Run("peer_a_after_peer_b_mention", func(t *testing.T) {
		waitUntil(t, stepTimeout, "peer-a reply after peer-b @", func(ctx context.Context) (bool, error) {
			events, err := reqMX.recentMessages(ctx, roomID)
			if err != nil {
				return false, err
			}
			return senderMessageCount(events, e.PeerAMXID) > peerAMid &&
				senderHasText(events, e.PeerAMXID, e.ExpectedText), nil
		})
	}) {
		return
	}
}

func waitMentionGroupSwarmReady(t *testing.T, e swarmEnv) string {
	t.Helper()
	cs := kubeClient(t)

	var roomID string
	waitUntil(t, stepTimeout, "swarm collaboration roomId", func(ctx context.Context) (bool, error) {
		v, err := configMapString(ctx, cs, e.Namespace, "mention-group-swarm-channel", "roomId")
		if err != nil {
			return false, err
		}
		if strings.Contains(v, "example.org") {
			return false, nil
		}
		roomID = v
		return true, nil
	})
	t.Logf("roomId=%s", roomID)

	for _, sec := range []string{"matrix-requester", "matrix-peer-a", "matrix-peer-b"} {
		name := sec
		waitUntil(t, stepTimeout, name+" secret", func(ctx context.Context) (bool, error) {
			_, err := secretString(ctx, cs, e.Namespace, name, "accessToken")
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return err == nil, err
		})
	}

	for _, member := range []string{"peer-a", "peer-b"} {
		name := member
		waitUntil(t, stepTimeout, "member "+name+" Ready", func(ctx context.Context) (bool, error) {
			phase, err := memberPhase(e.Namespace, name)
			if err != nil {
				return false, err
			}
			return phase == "Ready", nil
		})
	}

	return roomID
}
