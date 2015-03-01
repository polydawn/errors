package try_test

import (
	"fmt"

	"github.com/spacemonkeygo/errors"
	"github.com/spacemonkeygo/errors/try"
)

func ExampleNormalFlow() {
	try.Do(func() {
		fmt.Println("function called")
	}).Finally(func() {
		fmt.Println("finally block called")
	}).CatchAll(func(_ error) {
		fmt.Println("catch wildcard called")
	}).Done()

	// Output:
	// function called
	// finally block called
}

func ExampleErrorInTry() {
	try.Do(func() {
		fmt.Println("function called")
		panic(fmt.Errorf("any error"))
	}).Finally(func() {
		fmt.Println("finally block called")
	}).CatchAll(func(_ error) {
		fmt.Println("catch wildcard called")
	}).Done()

	// Output:
	// function called
	// catch wildcard called
	// finally block called
}

func ExampleCrashInCatch() {
	try.Do(func() {
		try.Do(func() {
			fmt.Println("function called")
			panic(fmt.Errorf("any error"))
		}).Finally(func() {
			fmt.Println("finally block called")
		}).CatchAll(func(_ error) {
			fmt.Println("catch wildcard called")
			panic(fmt.Errorf("zomg"))
		}).Done()
	}).CatchAll(func(e error) {
		fmt.Println("outer error caught:", e.Error())
	}).Done()

	// Output:
	// function called
	// catch wildcard called
	// finally block called
	// outer error caught: zomg
}

func ExampleErrorsLeaveFinally() {
	try.Do(func() {
		try.Do(func() {
			fmt.Println("function called")
			panic(fmt.Errorf("inner error"))
		}).Finally(func() {
			fmt.Println("finally block called")
		}).Done()
	}).CatchAll(func(e error) {
		fmt.Println("outer error caught:", e.Error())
	}).Done()

	// Output:
	// function called
	// finally block called
	// outer error caught: inner error
}

func ExampleCrashInFinally() {
	try.Do(func() {
		try.Do(func() {
			fmt.Println("function called")
		}).Finally(func() {
			fmt.Println("finally block called")
			panic(fmt.Errorf("zomg"))
		}).CatchAll(func(_ error) {
			fmt.Println("catch wildcard called")
		}).Done()
	}).CatchAll(func(e error) {
		fmt.Println("outer error caught:", e.Error())
	}).Done()

	// Output:
	// function called
	// finally block called
	// outer error caught: zomg
}

var FruitError = errors.NewClass("fruit")
var AppleError = FruitError.NewClass("apple")
var GrapeError = FruitError.NewClass("grape")
var RockError = errors.NewClass("rock")

func ExampleCatchingErrorsByType() {
	try.Do(func() {
		try.Do(func() {
			fmt.Println("function called")
			panic(AppleError.New("emsg"))
		}).Finally(func() {
			fmt.Println("finally block called")
		}).Catch(RockError, func(e *errors.Error) {
			fmt.Println("rock handler called")
		}).Catch(FruitError, func(e *errors.Error) {
			fmt.Println("fruit handler called")
		}).CatchAll(func(_ error) {
			fmt.Println("catch wildcard called")
		}).Done()
	}).CatchAll(func(e error) {
		fmt.Println("outer error caught:", e.Error())
	}).Done()

	// Output:
	// function called
	// fruit handler called
	// finally block called
}

func ExampleCatchingErrorsBySpecificType() {
	try.Do(func() {
		try.Do(func() {
			fmt.Println("function called")
			panic(AppleError.New("emsg"))
		}).Finally(func() {
			fmt.Println("finally block called")
		}).Catch(AppleError, func(e *errors.Error) {
			fmt.Println("apple handler called")
		}).Catch(FruitError, func(e *errors.Error) {
			fmt.Println("fruit handler called")
		}).CatchAll(func(_ error) {
			fmt.Println("catch wildcard called")
		}).Done()
	}).CatchAll(func(e error) {
		fmt.Println("outer error caught:", e.Error())
	}).Done()

	// Output:
	// function called
	// apple handler called
	// finally block called
}
