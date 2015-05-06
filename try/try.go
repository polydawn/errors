/*
	`try` provides idioms for scoped panic handling.

	It's specifically aware of spacemonkey errors and can easily create
	type-aware handling blocks.

	For a given block of code that is to be tried, several kinds of error
	handling are possible:
	  - `Catch(type, func(err) {...your handler...})`
	  - `CatchAll(func(err) {...your handler...})`
	  - `Finally(func() {...your handler...})`

	`Catch` and `CatchAll` blocks consume the error -- it will not be re-raised
	unless the handlers explicitly do so.  `Finally` blocks run even in the
	absense of errors (much like regular defers), and do not consume errors --
	they will be re-raised after the execution of the `Finally` block.

	Matching of errors occurs in order.  This has a few implications:
	  - If using `Catch` blocks with errors that are subclasses of other errors
	    you're handling in the same sequence, put the most specific ones first.
	  - `CatchAll` blocks should be last (or they'll eat all errors,
	    even if you declare more `Catch` blocks later).

	`Finally` blocks will be run at the end of the error handling sequence
	regardless of their declaration order.

	Additional panics from a `Catch` or `CatchAll` block will still cause
	`Finally` blocks to be executed.  However, note that additional panics
	raised from any handler blocks will cause the original error to be masked
	-- be careful of this.

	Panics with values that are not spacemonkey errors will be handled
	(no special treatment; they'll hit `CatchAll` blocks and `Finally` blocks;
	it would of course be silly for them to hit `Catch` blocks since those
	use spacemonkey-errors types).  Panics with values that are not of golang's
	`error` type at all will be wrapped in a spacemonkey error for the purpose
	of handling (as an interface design, `CatchAll(interface{})` is unpleasant).
	See `OriginalError` and `Repanic` for more invormation on handling these
	wrapped panic objects.

	A `try.Do(func() {...})` with no attached errors handlers is legal but
	pointless.  A `try.Do(func() {...})` with no `Done()` will never run the
	function (which is good; you won't forget to call it).

	For spacemonkey errors, the 'exit' path will be automatically recorded for
	each time the errors is rethrown.  This is not a complete record of where
	the error has been, and reexamining the current stack may give a more
	complete picture.  Note that 'finally' blocks will never be recorded
	(unless of course they raise a new error!), since they are functions that
	return normally.

	A note about use cases: while `try` should be familiar and comfortable
	to users of exceptions in other languages, and we feel use of a "typed"
	panic mechanism results in effective error handling with a minimization
	of boilerplate checking error return codes... `try` and `panic` are not
	always the correct answer!  There are at least two situations where you
	may want to consider converting back to handling errors as regular values:
	If passing errors between goroutines, of course you need to pass them
	as values.  Another situation where error returns are more capable than
	panics is when part of multiple-returns, where the other values may have
	been partially assembled and still may need be subject to cleanup by the
	caller.  Fortunately, you can always use `CatchAll` to easily fence a block
	of panic-oriented code and convert it into errors-by-value flow.
*/
package try

import (
	"fmt"

	"github.com/spacemonkeygo/errors"
)

var (
	// Panic type when a panic is caught that is neither a spacemonkey error, nor an ordinary golang error.
	// For example, panic("hooray!")
	UnknownPanicError = errors.NewClass("Unknown Error")

	// The spacemonkey error key to get the original data out of an UnknownPanicError.
	OriginalErrorKey = errors.GenSym()
)

type Plan struct {
	main    func()
	catch   []check
	finally func()
}

type check struct {
	match      *errors.ErrorClass
	handler    func(err *errors.Error)
	anyhandler func(err error)
}

func Do(f func()) *Plan {
	return &Plan{main: f, finally: func() {}}
}

func (p *Plan) Catch(kind *errors.ErrorClass, handler func(err *errors.Error)) *Plan {
	p.catch = append(p.catch, check{
		match:   kind,
		handler: handler,
	})
	return p
}

func (p *Plan) CatchAll(handler func(err error)) *Plan {
	p.catch = append(p.catch, check{
		match:      nil,
		anyhandler: handler,
	})
	return p
}

func (p *Plan) Finally(f func()) *Plan {
	f2 := p.finally
	p.finally = func() {
		f()
		f2()
	}
	return p
}

func (p *Plan) Done() {
	defer func() {
		rec := recover()
		consumed := false
		defer func() {
			p.finally()
			if !consumed {
				panic(rec)
			}
		}()
		switch err := rec.(type) {
		case nil:
			consumed = true
			return
		case *errors.Error:
			// record the origin location of the error.
			// this is redundant at first, but useful if the error is rethrown;
			// then it shows line of the panic that rethrew it.
			errors.RecordBefore(err, 3)
			// run all checks
			for _, catch := range p.catch {
				if catch.match == nil {
					consumed = true
					catch.anyhandler(err)
					return
				}
				if err.Is(catch.match) {
					consumed = true
					catch.handler(err)
					return
				}
			}
		case error:
			// grabbag error, so skip all the typed catches, but still do wildcards and finally.
			for _, catch := range p.catch {
				if catch.match == nil {
					consumed = true
					catch.anyhandler(err)
					return
				}
			}
		default:
			// handle the case where it's not even an error type.
			// we'll wrap your panic in an UnknownPanicError and add the original as data for later retrieval.
			for _, catch := range p.catch {
				if catch.match == nil {
					consumed = true
					msg := fmt.Sprintf("%v", rec)
					pan := UnknownPanicError.NewWith(msg, errors.SetData(OriginalErrorKey, rec))
					catch.anyhandler(pan)
					return
				}
				if UnknownPanicError.Is(catch.match) {
					consumed = true
					msg := fmt.Sprintf("%v", rec)
					pan := UnknownPanicError.NewWith(msg, errors.SetData(OriginalErrorKey, rec))
					catch.handler(pan.(*errors.Error))
					return
				}
			}
		}
	}()
	p.main()
}

/*
	If `err` was originally another value coerced to an error by `CatchAll`,
	this will return the original value.  Otherwise, it returns the same error
	again.

	See also `Repanic()`.
*/
func OriginalError(err error) interface{} {
	data := errors.GetData(err, OriginalErrorKey)
	if data == nil {
		return err
	}
	return data
}

/*
	Panics again with the original error.

	Shorthand, equivalent to calling `panic(OriginalError(err))`

	If the error is a `UnknownPanicError` (i.e., a `CatchAll` block that's handling something
	that wasn't originally an `error` type, so it was wrapped), it will unwrap that re-panic
	with that original error -- in other words, this is a "do the right thing" method in all scenarios.

	You may simply `panic(err)` for all other typed `Catch` blocks (since they are never wraps),
	though there's no harm in using `Repanic` consistently if you prefer.  It is also safe to
	use `Repanic` on other non-`error` types (though you're really not helping anyone with that
	behavior) and objects that didn't originally come in as the handler's error.  No part of
	the error handling or finally handlers chain will change execution order as a result;
	`panic` and `Repanic` cause identical behavior in that regard.
*/
func Repanic(err error) {
	wrapper, ok := err.(*errors.Error)
	if !ok {
		panic(err)
	}
	if !wrapper.Is(UnknownPanicError) {
		panic(err)
	}
	data := errors.GetData(err, OriginalErrorKey)
	if data == nil {
		panic(errors.ProgrammerError.New("misuse of try internals", errors.SetData(OriginalErrorKey, err)))
	}
	panic(data)
}
