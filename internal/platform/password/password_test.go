package password

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := Hash("a-strong-password")
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(hash, "a-strong-password") {
		t.Fatal("correct password was rejected")
	}
	if Verify(hash, "wrong-password") {
		t.Fatal("incorrect password was accepted")
	}
}
