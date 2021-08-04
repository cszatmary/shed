package spinner_test

import (
	"fmt"
	"testing"

	"github.com/getshiphub/shed/internal/spinner"
)

func TestTTYSpinner(t *testing.T) {
	out := &syncBuffer{}
	s := spinner.NewTTY(spinner.TTYOptions{
		Options: spinner.Options{
			Out:     out,
			Message: "Cloning repos",
		},
		IsaTTY: false,
	})
	s.Start()
	s.UpdateMessage("Updating repos")
	fmt.Fprint(s, "Some debug info")
	s.UpdateMessage("Cleaning up")
	s.Stop()

	got := out.String()
	want := "Cloning repos\nUpdating repos\nSome debug info\nCleaning up\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
