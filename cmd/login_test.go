package cmd

import "testing"

// ログインまわりのコマンドが登録されていて、3 バイナリすべてで使えることを確かめる。
func TestLoginCommandsAreRegistered(t *testing.T) {
	for _, name := range []string{"login", "logout", "me", "session-set-cookie"} {
		c, _, err := rootCmd.Find([]string{name})
		if err != nil || c == nil || c.Name() != name {
			t.Fatalf("command %q is not registered", name)
		}
		for _, p := range []string{ProfileGaroop, ProfileGaruchan, ProfileGaroopTV} {
			if !containsProfile(c.Annotations[annotationProfiles], p) {
				t.Errorf("command %q is not available in profile %q", name, p)
			}
		}
	}
	c, _, _ := rootCmd.Find([]string{"login"})
	if c.Flags().Lookup("code") == nil || c.Flags().Lookup("email") == nil {
		t.Error("login must have --email and --code")
	}
}
