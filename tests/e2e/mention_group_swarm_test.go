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

// TestMentionGroupSwarm covers two Groups and two Collaboration rooms:
//  1. mixed-oc (OpenClaw in mixed-team) @hermes-team in room A → hermes-a and hermes-b reply
//  2. add hermes-b to mixed-team → @mixed-team in room A also wakes hermes-b
//  3. remove hermes-b from hermes-team → room B @hermes-team does not wake it; it stays in room B
func TestMentionGroupSwarm(t *testing.T) {
	e := loadSwarmEnv()
	cs := kubeClient(t)
	ctx := context.Background()
	roomA, roomB := waitMentionGroupSwarmReady(t, e)

	requesterTok, err := secretString(ctx, cs, e.Namespace, "matrix-requester", "accessToken")
	if err != nil {
		t.Fatalf("requester token: %v", err)
	}
	mixedOCTok, err := secretString(ctx, cs, e.Namespace, "matrix-mixed-oc", "accessToken")
	if err != nil {
		t.Fatalf("mixed-oc token: %v", err)
	}

	reqMX := &matrixClient{base: e.MatrixURL, token: requesterTok, http: &http.Client{Timeout: 20 * time.Second}}
	mixedOCMX := &matrixClient{base: e.MatrixURL, token: mixedOCTok, http: &http.Client{Timeout: 20 * time.Second}}

	beforeA, err := reqMX.recentMessages(ctx, roomA)
	if err != nil {
		t.Fatalf("list room A: %v", err)
	}
	hermesABefore := senderMessageCount(beforeA, e.HermesAMXID)
	hermesBBefore := senderMessageCount(beforeA, e.HermesBMXID)
	mixedHMBefore := senderMessageCount(beforeA, e.MixedHMMXID)

	if err := mixedOCMX.sendMention(ctx, roomA, "E2E_SWARM_GROUP_AT_GROUP please reply", []string{e.HermesTeamMXID}); err != nil {
		t.Fatalf("mixed-oc @hermes-team: %v", err)
	}

	if !t.Run("mixed_oc_at_hermes_team", func(t *testing.T) {
		waitUntil(t, stepTimeout, "hermes-a and hermes-b reply after mixed-oc @hermes-team", func(ctx context.Context) (bool, error) {
			events, err := reqMX.recentMessages(ctx, roomA)
			if err != nil {
				return false, err
			}
			aOK := senderMessageCount(events, e.HermesAMXID) > hermesABefore &&
				senderHasText(events, e.HermesAMXID, e.ExpectedText)
			bOK := senderMessageCount(events, e.HermesBMXID) > hermesBBefore &&
				senderHasText(events, e.HermesBMXID, e.ExpectedText)
			return aOK && bOK, nil
		})
		events, err := reqMX.recentMessages(ctx, roomA)
		if err != nil {
			t.Fatalf("list room A: %v", err)
		}
		if senderMessageCount(events, e.MixedHMMXID) > mixedHMBefore {
			t.Fatal("mixed-hm replied after @hermes-team; want hermes-team only")
		}
	}) {
		return
	}

	if !t.Run("add_hermes_b_to_mixed_team", func(t *testing.T) {
		beforeAdd, err := reqMX.recentMessages(ctx, roomA)
		if err != nil {
			t.Fatalf("list room A: %v", err)
		}
		hermesBMid := senderMessageCount(beforeAdd, e.HermesBMXID)

		if err := kubectlReplaceGroupMembers(e.Namespace, "mixed-team", []string{"mixed-oc", "mixed-hm", "hermes-b"}); err != nil {
			t.Fatalf("add hermes-b to mixed-team: %v", err)
		}

		waitUntil(t, stepTimeout, "hermes-b in room A after mixed-team add", func(ctx context.Context) (bool, error) {
			joined, err := reqMX.joinedMemberIDs(ctx, roomA)
			if err != nil {
				return false, err
			}
			return joinedHas(joined, e.HermesBMXID), nil
		})

		if err := reqMX.sendMention(ctx, roomA, "E2E_SWARM_MIXED_AFTER_ADD please reply", []string{e.MixedTeamMXID}); err != nil {
			t.Fatalf("human @mixed-team: %v", err)
		}
		waitUntil(t, stepTimeout, "hermes-b reply after added to mixed-team", func(ctx context.Context) (bool, error) {
			events, err := reqMX.recentMessages(ctx, roomA)
			if err != nil {
				return false, err
			}
			return senderMessageCount(events, e.HermesBMXID) > hermesBMid &&
				senderHasText(events, e.HermesBMXID, e.ExpectedText), nil
		})
	}) {
		return
	}

	if !t.Run("remove_hermes_b_from_hermes_team", func(t *testing.T) {
		beforeB, err := reqMX.recentMessages(ctx, roomB)
		if err != nil {
			t.Fatalf("list room B: %v", err)
		}
		hermesARoomB := senderMessageCount(beforeB, e.HermesAMXID)
		hermesBRoomB := senderMessageCount(beforeB, e.HermesBMXID)

		if err := kubectlReplaceGroupMembers(e.Namespace, "hermes-team", []string{"hermes-a"}); err != nil {
			t.Fatalf("remove hermes-b from hermes-team: %v", err)
		}

		if err := reqMX.sendMention(ctx, roomB, "E2E_SWARM_HERMES_AFTER_REMOVE please reply", []string{e.HermesTeamMXID}); err != nil {
			t.Fatalf("human @hermes-team in room B: %v", err)
		}
		waitUntil(t, stepTimeout, "hermes-a reply after hermes-b removed", func(ctx context.Context) (bool, error) {
			events, err := reqMX.recentMessages(ctx, roomB)
			if err != nil {
				return false, err
			}
			return senderMessageCount(events, e.HermesAMXID) > hermesARoomB &&
				senderHasText(events, e.HermesAMXID, e.ExpectedText), nil
		})

		deadline := time.Now().Add(45 * time.Second)
		for time.Now().Before(deadline) {
			events, err := reqMX.recentMessages(ctx, roomB)
			if err != nil {
				t.Fatalf("list room B: %v", err)
			}
			if senderMessageCount(events, e.HermesBMXID) > hermesBRoomB {
				t.Fatal("hermes-b replied after being removed from hermes-team")
			}
			time.Sleep(3 * time.Second)
		}

		joined, err := reqMX.joinedMemberIDs(ctx, roomB)
		if err != nil {
			t.Fatalf("joined members room B: %v", err)
		}
		if !joinedHas(joined, e.HermesBMXID) {
			t.Fatal("hermes-b missing from room B after group removal; want stay (no kick)")
		}
	}) {
		return
	}
}

func waitMentionGroupSwarmReady(t *testing.T, e swarmEnv) (roomA, roomB string) {
	t.Helper()
	cs := kubeClient(t)

	waitUntil(t, stepTimeout, "swarm collaboration room A", func(ctx context.Context) (bool, error) {
		v, err := configMapString(ctx, cs, e.Namespace, "mention-group-swarm-channel", "roomId")
		if err != nil {
			return false, err
		}
		if strings.Contains(v, "example.org") {
			return false, nil
		}
		roomA = v
		return true, nil
	})
	waitUntil(t, stepTimeout, "hermes collaboration room B", func(ctx context.Context) (bool, error) {
		v, err := configMapString(ctx, cs, e.Namespace, "mention-group-hermes-channel", "roomId")
		if err != nil {
			return false, err
		}
		if strings.Contains(v, "example.org") {
			return false, nil
		}
		roomB = v
		return true, nil
	})
	t.Logf("roomA=%s roomB=%s", roomA, roomB)

	for _, sec := range []string{"matrix-requester", "matrix-mixed-oc", "matrix-mixed-hm", "matrix-hermes-a", "matrix-hermes-b"} {
		name := sec
		waitUntil(t, stepTimeout, name+" secret", func(ctx context.Context) (bool, error) {
			_, err := secretString(ctx, cs, e.Namespace, name, "accessToken")
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return err == nil, err
		})
	}

	for _, member := range []string{"mixed-oc", "mixed-hm", "hermes-a", "hermes-b"} {
		name := member
		waitUntil(t, stepTimeout, "member "+name+" Ready", func(ctx context.Context) (bool, error) {
			phase, err := memberPhase(e.Namespace, name)
			if err != nil {
				return false, err
			}
			return phase == "Ready", nil
		})
	}

	requesterTok, err := secretString(context.Background(), cs, e.Namespace, "matrix-requester", "accessToken")
	if err != nil {
		t.Fatalf("requester token: %v", err)
	}
	reqMX := &matrixClient{base: e.MatrixURL, token: requesterTok, http: &http.Client{Timeout: 20 * time.Second}}
	waitUntil(t, stepTimeout, "requester joined room A and B", func(ctx context.Context) (bool, error) {
		a, err := reqMX.joinedMemberIDs(ctx, roomA)
		if err != nil {
			return false, err
		}
		b, err := reqMX.joinedMemberIDs(ctx, roomB)
		if err != nil {
			return false, err
		}
		needA := []string{e.MixedOCMXID, e.HermesAMXID, e.HermesBMXID, e.HermesTeamMXID}
		for _, mxid := range needA {
			if !joinedHas(a, mxid) {
				return false, nil
			}
		}
		return joinedHas(b, e.HermesAMXID) && joinedHas(b, e.HermesBMXID), nil
	})

	return roomA, roomB
}
