package keyxfer

import (
	"errors"
	"strings"
	"testing"
)

const testPub = "age1qqqexamplelocalpublickey000000000000000000000000000"

// stubSSH installs an sshRun that answers inspect / mkdir / chmod / verify, and
// a scpUpload that records what was uploaded. existingPub/hasKey shape the
// target's pre-state; verifyPub is what the post-write read-back returns.
func stubSSH(t *testing.T, existingPub string, hasKey bool, verifyPub string) *[]string {
	t.Helper()
	origSSH, origSCP := sshRun, scpUpload
	t.Cleanup(func() { sshRun, scpUpload = origSSH, origSCP })

	var uploads []string
	scpUpload = func(localPath, host, remotePath string) error {
		uploads = append(uploads, remotePath)
		return nil
	}
	sshRun = func(_ string, command string) (string, int, error) {
		switch {
		case strings.Contains(command, inspectMarker):
			out := existingPub + "\n" + inspectMarker + "\n"
			if hasKey {
				out += "yes\n"
			}
			return out, 0, nil
		case strings.Contains(command, "mkdir"), strings.Contains(command, "printf"):
			return "", 0, nil
		default: // the read-back verify
			return verifyPub, 0, nil
		}
	}
	return &uploads
}

func TestPushProvisionsFreshTarget(t *testing.T) {
	uploads := stubSSH(t, "", false, testPub)
	outcome, err := Push("user@host", "/local/identity.age.key", testPub, false)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if outcome != Provisioned {
		t.Errorf("outcome = %v, want Provisioned", outcome)
	}
	if len(*uploads) != 1 || (*uploads)[0] != remoteKeyPath {
		t.Errorf("expected the private key uploaded to %s, got %v", remoteKeyPath, *uploads)
	}
}

func TestPushAlreadyPresentIsNoOp(t *testing.T) {
	uploads := stubSSH(t, testPub, true, testPub)
	outcome, err := Push("user@host", "/local/identity.age.key", testPub, false)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if outcome != AlreadyPresent {
		t.Errorf("outcome = %v, want AlreadyPresent", outcome)
	}
	if len(*uploads) != 0 {
		t.Errorf("should not upload when already present, got %v", *uploads)
	}
}

func TestPushRefusesDifferentIdentity(t *testing.T) {
	uploads := stubSSH(t, "age1qqqsomeotherkey99999999999999999999999999999999999", true, testPub)
	_, err := Push("user@host", "/local/identity.age.key", testPub, false)
	if !errors.Is(err, ErrDifferentIdentity) {
		t.Fatalf("expected ErrDifferentIdentity, got %v", err)
	}
	if len(*uploads) != 0 {
		t.Errorf("should not upload when refusing, got %v", *uploads)
	}
}

func TestPushForceOverwritesDifferentIdentity(t *testing.T) {
	stubSSH(t, "age1qqqsomeotherkey99999999999999999999999999999999999", true, testPub)
	outcome, err := Push("user@host", "/local/identity.age.key", testPub, true)
	if err != nil {
		t.Fatalf("Push --force: %v", err)
	}
	if outcome != Provisioned {
		t.Errorf("outcome = %v, want Provisioned", outcome)
	}
}

func TestPushUnreachable(t *testing.T) {
	origSSH := sshRun
	t.Cleanup(func() { sshRun = origSSH })
	sshRun = func(_, _ string) (string, int, error) {
		return "Host key verification failed.", 255, nil
	}
	_, err := Push("user@host", "/local/identity.age.key", testPub, false)
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("expected ErrUnreachable, got %v", err)
	}
}

func TestPushVerificationMismatch(t *testing.T) {
	stubSSH(t, "", false, "age1qqqwrongreadback00000000000000000000000000000000000")
	_, err := Push("user@host", "/local/identity.age.key", testPub, false)
	if err == nil || !strings.Contains(err.Error(), "verification failed") {
		t.Fatalf("expected verification failure, got %v", err)
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("age1abc"); got != "'age1abc'" {
		t.Errorf("shellQuote = %q", got)
	}
	if got := shellQuote("a'b"); got != `'a'\''b'` {
		t.Errorf("shellQuote with quote = %q", got)
	}
}
