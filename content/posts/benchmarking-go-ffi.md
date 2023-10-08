---
title: "Benchmarking Go FFI"
date: 2023-04-11T21:27:49+02:00
---

All programming languages offer a way to call C function and libraries, via a mechanism called FFI (Foreign Function Interface).
This allows for compatibility with code written in a different programming language or in some case to make certain operation faster (This is what numpy does in python).

There are contradictory sources on the internet that claims that CGO (the GO FFI name) is either slow or fast enough so it doesn't matter. 

Let's measure the actually cost by running some benchmarks.

# First, a baseline of the cost of a function call

We'll use a very simple pure go function that adds numbers:
```go
func add(x, y uint64) uint64{
  return x + y
}
```

And the associated benchmark:
```go
func BenchmarkAddPureGo(b *testing.B) {
	x := uint64(0x12345678)
	y := uint64(0xABCDEF00)
	for i := 0; i < b.N; i++ {
		add(x, y)
	}
}
```


Let's run it:
```sh
❯ go test . -bench=.
goos: linux
goarch: amd64
pkg: cgo-bench
cpu: 11th Gen Intel(R) Core(TM) i7-1185G7 @ 3.00GHz
BenchmarkAddPureGo-8	1000000000			0.2176 ns/op
PASS
ok		cgo-bench	0.245s
```

The benchmark ran in 0.245 seconds, and executed `add` 1000000000 times, which means that each execution took 0.2176 ns.
Given that my processor runs at 3GHz, or 0.33 ns per cycle, this number is suspiciously low.
Maybe the compiler optimized away most of our benchmark ?

It's possible to verify that by looking at the assembly produced for the benchmark:

```sh
❯ go test -c . # create an executable file
❯ go tool objdump -gnu -s BenchmarkAddPureGo cgo-bench.test
TEXT cgo-bench.BenchmarkAddPureGo(SB) /home/aurelien/personnal/cgo-bench/main_test.go
  0x4f8480		31c9			XORL CX, CX                          // xor %ecx,%ecx
  0x4f8482		eb03			JMP 0x4f8487                         // jmp 0x4f8487
  0x4f8484		48ffc1			INCQ CX                              // inc %rcx
  0x4f8487		483988a0010000	CMPQ CX, 0x1a0(AX)                   // cmp %rcx,0x1a0(%rax)
  0x4f848e		7ff4			JG 0x4f8484                          // jg 0x4f8484
  0x4f8490		c3				RET                                  // retq
```

There's no `CALL` or `ADD` instruction here.
It seems that the compiler realised that he was calling a function that had no side effect and those result was never used and thus decided to remove it completely.
What remains is the code of the loop itself that increments a variable until it reaches a certain value.

This kind of behavior is usually good to have in compiler because it make the programs faster, but here we don't want that.
This behavior can be prevented by storing the result of the `add` function in a global variable.
This makes it more difficult for the compiler to figure out that the result is never used, and may prevent this optimization to happen.

Let's try with this new benchmark function:
```go
var result uint64

func BenchmarkAddPureGo(b *testing.B) {
	x := uint64(0x12345678)
	y := uint64(0xABCDEF00)
	for i := 0; i < b.N; i++ {
		y = add(x, y)
	}
	result = y
}
```

```sh
❯ go test -c .
❯ go tool objdump -gnu -s BenchmarkAddPureGo cgo-bench.test
TEXT cgo-bench.BenchmarkAddPureGo(SB) /home/aurelien/personnal/cgo-bench/main_test.go
  main_test.go:19	0x4f8480		31c9			XORL CX, CX                          // xor %ecx,%ecx
  main_test.go:19	0x4f8482		ba00efcdab		MOVL $-0x54321100, DX                // mov $-0x54321100,%edx
  main_test.go:22	0x4f8487		eb0b			JMP 0x4f8494                         // jmp 0x4f8494
  main_test.go:22	0x4f8489		48ffc1			INCQ CX                              // inc %rcx
  main_test.go:23	0x4f848c		90				NOPL                                 // nop
  main_test.go:6	0x4f848d		4881c278563412	ADDQ $0x12345678, DX                 // add $0x12345678,%rdx
  main_test.go:22	0x4f8494		483988a0010000	CMPQ CX, 0x1a0(AX)                   // cmp %rcx,0x1a0(%rax)
  main_test.go:22	0x4f849b		7fec			JG 0x4f8489                          // jg 0x4f8489
  main_test.go:25	0x4f849d		48891524521400	MOVQ DX, cgo-bench.result(SB)        // mov %rdx,0x145224(%rip)
  main_test.go:26	0x4f84a4		c3				RET                                  // retq
```

The `ADDQ` instruction is now present in the generated assembly, so code does add the `0x12345678` constant to some register in a loop.
However there is no `CALL` instruction to be seen here.
This is because an other optimization was done by the compiler: it **inlined** the function call.
Inlining is a optimization where the compiler replace the function call by the code of the function itself.
This removes the overhead of the calling of the function, so that the cpu can spend all of it's time actually running the code inside the function.

This optimization can be disabled by using the `go:noinline` magic comment.
It instructs the compiler to never optimize the function.

```go
//go:noinline
func add(x, y uint64) uint64 {
	return x + y
}
```

And now the compiler doesn't inline the function anymore:

```sh
❯ go test -c .
❯ go tool objdump -gnu -s BenchmarkAddPureGo cgo-bench.test
TEXT cgo-bench.BenchmarkAddPureGo(SB) /home/aurelien/personnal/cgo-bench/main_test.go
  main_test.go:20	0x4f84a0		493b6610		CMPQ 0x10(R14), SP                   // cmp 0x10(%r14),%rsp
  main_test.go:20	0x4f84a4		7658			JBE 0x4f84fe                         // jbe 0x4f84fe
  main_test.go:20	0x4f84a6		4883ec20		SUBQ $0x20, SP                       // sub $0x20,%rsp
  main_test.go:20	0x4f84aa		48896c2418		MOVQ BP, 0x18(SP)                    // mov %rbp,0x18(%rsp)
  main_test.go:20	0x4f84af		488d6c2418		LEAQ 0x18(SP), BP                    // lea 0x18(%rsp),%rbp
  main_test.go:20	0x4f84b4		4889442428		MOVQ AX, 0x28(SP)                    // mov %rax,0x28(%rsp)
  main_test.go:20	0x4f84b9		31c9			XORL CX, CX                          // xor %ecx,%ecx
  main_test.go:20	0x4f84bb		ba00efcdab		MOVL $-0x54321100, DX                // mov $-0x54321100,%edx
  main_test.go:23	0x4f84c0		eb22			JMP 0x4f84e4                         // jmp 0x4f84e4
  main_test.go:23	0x4f84c2		48894c2410		MOVQ CX, 0x10(SP)                    // mov %rcx,0x10(%rsp)
  main_test.go:24	0x4f84c7		b878563412		MOVL $0x12345678, AX                 // mov $0x12345678,%eax
  main_test.go:24	0x4f84cc		4889d3			MOVQ DX, BX                          // mov %rdx,%rbx
  main_test.go:24	0x4f84cf		e8acffffff		CALL cgo-bench.add(SB)               // callq 0x4f8480
  main_test.go:23	0x4f84d4		488b4c2410		MOVQ 0x10(SP), CX                    // mov 0x10(%rsp),%rcx
  main_test.go:23	0x4f84d9		48ffc1			INCQ CX                              // inc %rcx
  main_test.go:26	0x4f84dc		4889c2			MOVQ AX, DX                          // mov %rax,%rdx
  main_test.go:23	0x4f84df		488b442428		MOVQ 0x28(SP), AX                    // mov 0x28(%rsp),%rax
  main_test.go:23	0x4f84e4		483988a0010000	CMPQ CX, 0x1a0(AX)                   // cmp %rcx,0x1a0(%rax)
  main_test.go:23	0x4f84eb		7fd5			JG 0x4f84c2                          // jg 0x4f84c2
  main_test.go:26	0x4f84ed		488915d4511400	MOVQ DX, cgo-bench.result(SB)        // mov %rdx,0x1451d4(%rip)
  main_test.go:27	0x4f84f4		488b6c2418		MOVQ 0x18(SP), BP                    // mov 0x18(%rsp),%rbp
  main_test.go:27	0x4f84f9		4883c420		ADDQ $0x20, SP                       // add $0x20,%rsp
  main_test.go:27	0x4f84fd		c3				RET                                  // retq
  main_test.go:20	0x4f84fe		4889442408		MOVQ AX, 0x8(SP)                     // mov %rax,0x8(%rsp)
  main_test.go:20	0x4f8503		e818cff6ff		CALL runtime.morestack_noctxt.abi0(SB) // callq 0x465420
  main_test.go:20	0x4f8508		488b442408		MOVQ 0x8(SP), AX                     // mov 0x8(%rsp),%rax
  main_test.go:20	0x4f850d		eb91			JMP cgo-bench.BenchmarkAddPureGo(SB) // jmp 0x4f84a0

❯ go tool objdump -gnu -s cgo-bench.add  cgo-bench.test
TEXT cgo-bench.add(SB) /home/aurelien/personnal/cgo-bench/main_test.go
  main_test.go:7	0x4f8480		4801d8			ADDQ BX, AX                          // add %rbx,%rax
  main_test.go:7	0x4f8483		c3				RET                                  // retq
```

The `CALL` to `cgo-bench.add` if indeed here.
There's also a bit of boilerplate around to abide to the go calling convention and to put the function parameters in the correct registers `%rax` and `%rbx`

Now the benchmark should give a better result for the cost of a function call:
```sh
❯ go test . -bench=BenchmarkAddPureGo
goos: linux
goarch: amd64
pkg: cgo-bench
cpu: 11th Gen Intel(R) Core(TM) i7-1185G7 @ 3.00GHz
BenchmarkAddPureGo-8	1000000000			0.9436 ns/op
PASS
ok		cgo-bench	1.052s
```

# The CGO version

Let's write a CGO version of our `add` function, that actually calls and `add` function defined in C:
It's possible to write C code in a go file directly by writing in in the comments above the `import "C"` statement.

```go
package cgo_bench

/*
   #include <stdint.h>

   uint64_t add(uint64_t a, uint64_t b) {
	  return a + b;
   }
*/
import "C"

func addcgo(x, y uint64) uint64 {
	x_c := C.uint64_t(x)
	y_c := C.uint64_t(y)
	sum, err := C.add(x_c, y_c)
	if err != nil {
		panic("faild to call C add implementation ")
	}
	return uint64(sum)
}

```

TODO: make this a real note somehow

Note that this code needs to be in a different file as we can't `import "C"` from test files due to
a limitation of the go toolchain: https://github.com/golang/go/issues/20381


Here is the associated benchmark
```go
func BenchmarkAddCGo(b *testing.B) {
	x := uint64(0x12345678)
	y := uint64(0xABCDEF00)
	for i := 0; i < b.N; i++ {
		y = addcgo(x, y)
	}
	result = y
}
```

And it's result:
```
❯ go test . -bench=BenchmarkAddCGo
goos: linux
goarch: amd64
pkg: cgo-bench
cpu: 11th Gen Intel(R) Core(TM) i7-1185G7 @ 3.00GHz
BenchmarkAddCGo-8		29442754		   40.70 ns/op
PASS
ok		cgo-bench	1.205s
```

Using the cgo version is indeed slower: it takes around 40 nanoseconds for to process each call to `addcgo`

# Conclusion

Using the Go FFI to call a C function has an overhead of approximately 40 nanoseconds.
For very trivial function that takes a few nanoseconds to execute like in this benchmark, it creates a serious slowdown.
However, in most practical cases, this overhead will probably be marginal.
If the C function takes 1 µs to execute, adding 40 ns of CGO overhead only represents a 4% overhead.
And if it takes 1ms, it's only 0.004% overhead.

Using CGO also has an other downside: it's impossible for the compiler to inline those C function, and thus can prevent some of the compiler optimizations to take place.
But once again if the C function are large enough, this shouldn't impact the performance much.

Thus, using CGO in your next project will likely not create a performance issue.

As a closing thought, the benchmark here only measure the overhead for calling CGO in a single thread.
It seems that there is also some contention issues if you use CGO on large multicore servers
https://shane.ai/posts/cgo-performance-in-go1.21/
