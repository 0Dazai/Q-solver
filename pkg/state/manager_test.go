package state

import "testing"

func TestApplyStealthTransitionKeepsLocalWindowVisible(t *testing.T) {
	current := WindowState{Visible: true, StealthMode: true}

	next := applyStealthTransition(current, false)

	if !next.Visible {
		t.Fatal("切换录屏隔离不应隐藏本机窗口")
	}
	if next.StealthMode {
		t.Fatal("隐身模式应切换为关闭")
	}
}
