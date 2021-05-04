package pkg

import (
	"runtime"
	"testing"
)

func Test_execCmd(t *testing.T) {
	switch runtime.GOOS {
	case "windows":
		output, err := execCmd(".", "git", "version")
		if err != nil {
			t.Error("test fails", err)
		}
		if string(output) != "git version 2.31.1.windows.1\n" {
			t.Errorf("output should be git version 2.31.1.windows.1, current %q", output)
		}
		
		//t.Error("this test does not work on windows")
	default:
		output, err := execCmd(".", "echo", "-n", "test")
		if err != nil {
			t.Error("test fails", err)
		}
		if string(output) != "test" {
			t.Errorf("output should be test, current %q", output)
		}
	}
}