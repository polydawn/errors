package try_test

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spacemonkeygo/errors"
	"github.com/spacemonkeygo/errors/try"
)

func TestStackHandling(t *testing.T) {
	// Crappy test.  Try harder later.
	try.Do(func() {
		try.Do(func() {
			fmt.Println("function called")
			panic(AppleError.New("emsg"))
		}).Finally(func() {
			fmt.Println("finally block called")
		}).Catch(FruitError, func(e *errors.Error) {
			fmt.Println("fruit handler called")
			panic(e)
		}).Done()
	}).CatchAll(func(e error) {
		fmt.Println("exit route:")
		fmt.Println(errors.GetExits(e))
		fmt.Println()
		fmt.Println("recorded stack:")
		fmt.Println(errors.GetStack(e))
		fmt.Println()

		fmt.Println("final stack:")
		var pcs [256]uintptr
		amount := runtime.Callers(3, pcs[:])
		for i := 0; i < amount; i++ {
			fmt.Println(frameStringer(pcs[i]))
		}
		fmt.Println()

	}).Done()
}

func frameStringer(pc uintptr) string {
	if pc == 0 {
		return "unknown.unknown:0"
	}
	f := runtime.FuncForPC(pc)
	if f == nil {
		return "unknown.unknown:0"
	}
	file, line := f.FileLine(pc)
	return fmt.Sprintf("%s:%s:%d", f.Name(), filepath.Base(file), line)
}
