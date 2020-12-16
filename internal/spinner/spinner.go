package spinner

import (
	"fmt"
	"strings"
	"time"

	"github.com/cszatmary/shed/internal/util"
	log "github.com/sirupsen/logrus"
)

var fatal = util.Fatal{}

func spinnerBar(total int) func(int) {
	spinnerFrames := []string{"|", "/", "-", "\\"}
	progress := 0
	animState := 0
	return func(inc int) {
		progress += inc
		var bar strings.Builder
		bar.WriteString("\r")
		bar.WriteString(spinnerFrames[animState])
		bar.WriteString(" [")
		for i := 0; i < total; i++ {
			if progress > i {
				bar.WriteString("#")
			} else {
				bar.WriteString("-")
			}
		}
		bar.WriteString("]")
		animState++
		animState = animState % len(spinnerFrames)
		fmt.Print(bar.String())
		if progress == total {
			clearLine(total + 4)
		}
	}
}

func clearLine(length int) {
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteString(" ")
	}
	fmt.Printf("\r")
	fmt.Print(b.String())
	fmt.Printf("\r")
}

func SpinnerWait(successCh chan string, failedCh chan error, successMsg string, failedMsg string, count int) {
	spin := spinnerBar(count)
	for i := 0; i < count; {
		select {
		case name := <-successCh:
			if !log.IsLevelEnabled(log.DebugLevel) {
				clearLine(count + 4)
			}
			log.Infof(successMsg, name)
			i++
			if !log.IsLevelEnabled(log.DebugLevel) {
				spin(1)
			}
		case err := <-failedCh:
			fmt.Printf("\r\n")
			fatal.ExitErrf(err, failedMsg)
		case <-time.After(time.Second / 10):
			if !log.IsLevelEnabled(log.DebugLevel) {
				spin(0)
			}
		}
	}
}
