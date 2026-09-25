package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/term"
)

// readSecret はパスワード・合言葉・Cookie を本人から受け取る。
// 端末で実行されていれば伏せ字で入力してもらう。AIエージェント（Claude Code など）から
// 実行されて端末がないときは、macOS なら入力ダイアログを出して本人に入れてもらう。
// どちらの場合も値はエージェントの画面・コマンド引数・履歴に残らない。
// 値をパイプで渡された場合（CI など）はそれを読む。
func readSecret(label string) (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "%s: ", label)
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	if v, ok := readPipedStdin(); ok {
		return v, nil
	}
	if runtime.GOOS == "darwin" {
		return askWithDialog(label, true)
	}
	return "", fmt.Errorf("%sを入力できません。ユーザー本人が自分の端末で実行してください", label)
}

// readPlain は伏せる必要のない値（メールアドレスなど）を受け取る。
func readPlain(label string) (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "%s: ", label)
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}
	if runtime.GOOS == "darwin" {
		return askWithDialog(label, false)
	}
	return "", fmt.Errorf("%sを指定してください", label)
}

// readPipedStdin はパイプやファイルから渡された1行を読む。何も渡されていなければ ok=false。
func readPipedStdin() (string, bool) {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&(os.ModeNamedPipe) == 0 && !info.Mode().IsRegular() {
		return "", false
	}
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	line = strings.TrimSpace(line)
	return line, line != ""
}

// askWithDialog は macOS の入力ダイアログで値を受け取る。label は固定文言だけを渡す。
func askWithDialog(label string, hidden bool) (string, error) {
	script := fmt.Sprintf(`text returned of (display dialog %q default answer "" with title "Garoop" buttons {"キャンセル", "OK"} default button "OK" cancel button "キャンセル"`, label+"を入力してください（AIエージェントには見えません）")
	if hidden {
		script += " with hidden answer"
	}
	script += ")"
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("入力がキャンセルされました")
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
