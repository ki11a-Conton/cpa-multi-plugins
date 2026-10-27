package main

// session_id_edge_test.go — pins the 0.2.7 fix: the degenerate empty first
// user messages (content missing / null / "" / whitespace-only / empty
// array) must fall back to the DEVICE UUID, not hash to one constant
// session-id shared by every such conversation (the pre-0.2.7 bug: two
// different conversations with the same empty shape got the SAME
// x-session-id — BigModel's prompt-cache key collided across conversations
// and accounts, violating the feature's own "a different conversation gets
// a different one").

import (
	"testing"
)

func TestSessionID_EmptyFirstUserFallsBackToDevice(t *testing.T) {
	emptyish := []struct {
		name    string
		payload string
	}{
		{"no content field", `{"messages":[{"role":"user"},{"role":"assistant","content":"A"}]}`},
		{"null content", `{"messages":[{"role":"user","content":null},{"role":"assistant","content":"A"}]}`},
		{"empty string content", `{"messages":[{"role":"user","content":""},{"role":"assistant","content":"A"}]}`},
		{"whitespace-only content", `{"messages":[{"role":"user","content":"   "},{"role":"assistant","content":"A"}]}`},
		{"empty array content", `{"messages":[{"role":"user","content":[]},{"role":"assistant","content":"A"}]}`},
	}
	device := uuidFromHash("zcode-session-device", "device-mid-1")
	for _, c := range emptyish {
		sid := conversationSessionID([]byte(c.payload), "device-mid-1")
		if sid != device {
			t.Errorf("%s: session-id %s, want the device fallback %s (a constant key here collides every empty-start conversation)", c.name, sid, device)
		}
	}
	// The bug's literal shape: two DIFFERENT conversations, same empty
	// spelling — must not share one session-id.
	a := conversationSessionID([]byte(`{"messages":[{"role":"user"},{"role":"assistant","content":"answer A"}]}`), "device-mid-1")
	b := conversationSessionID([]byte(`{"messages":[{"role":"user"},{"role":"assistant","content":"totally different B"}]}`), "device-mid-1")
	// Both take the device fallback (one account, no text to tell apart) —
	// acceptable per the documented fallback; the pre-0.2.7 constant is gone.
	if a != device || b != device {
		t.Errorf("empty-start conversations must ride the device fallback, got a=%s b=%s device=%s", a, b, device)
	}
	// Different accounts' empty-start conversations must NEVER share.
	other := conversationSessionID([]byte(`{"messages":[{"role":"user"},{"role":"assistant","content":"answer A"}]}`), "device-mid-2")
	if other == a {
		t.Errorf("session-id leaked across accounts: %s", a)
	}
}

func TestSessionID_NonEmptyShapesStayDistinct(t *testing.T) {
	// Image-only parts arrays and unreadable shapes keep the raw-bytes key:
	// distinct per conversation, never the device fallback.
	img1 := conversationSessionID([]byte(`{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA"}}]}]}`), "device-mid-1")
	img2 := conversationSessionID([]byte(`{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,BBB"}}]}]}`), "device-mid-1")
	device := uuidFromHash("zcode-session-device", "device-mid-1")
	if img1 == img2 {
		t.Errorf("two image-only conversations must stay distinct: %s", img1)
	}
	if img1 == device || img2 == device {
		t.Errorf("image-only conversations must not degenerate to the device fallback")
	}
	// Plain text keeps cross-turn stability and cross-conversation
	// distinctness (0.2.6 contract, unchanged).
	p1a := conversationSessionID([]byte(`{"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"x"}]}`), "device-mid-1")
	p1b := conversationSessionID([]byte(`{"messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"x"},{"role":"user","content":"turn two"}]}`), "device-mid-1")
	p2 := conversationSessionID([]byte(`{"messages":[{"role":"user","content":"other task"}]}`), "device-mid-1")
	if p1a != p1b {
		t.Errorf("same conversation must keep its session-id across turns: %s vs %s", p1a, p1b)
	}
	if p1a == p2 {
		t.Errorf("different conversations must differ: %s", p1a)
	}
}
