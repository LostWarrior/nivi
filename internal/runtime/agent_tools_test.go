package runtime

import "testing"

func TestDecodeToolArgsRejectsTrailingTopLevelTokens(t *testing.T) {
	t.Parallel()

	var args applyPatchArgs
	err := decodeToolArgs(`{"path":"a.txt","old_text":"a","new_text":"b"}{"extra":true}`, &args)
	if err == nil {
		t.Fatal("decodeToolArgs() expected error for trailing top-level token")
	}
}

func TestDecodeToolArgsAcceptsSingleObject(t *testing.T) {
	t.Parallel()

	var args applyPatchArgs
	err := decodeToolArgs(`{"path":"a.txt","old_text":"a","new_text":"b"}`, &args)
	if err != nil {
		t.Fatalf("decodeToolArgs() error = %v", err)
	}
}
