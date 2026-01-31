# Go Coding Guidelines

This document provides idiomatic Go patterns and conventions based on [Effective Go](https://go.dev/doc/effective_go) and the [Google Go Style Guide](https://google.github.io/styleguide/go/guide).

## Style Principles

Prioritize these attributes of readable code (in order of importance):

1. **Clarity**: Code's purpose and rationale is clear to the reader
2. **Simplicity**: Accomplishes goals in the simplest way possible
3. **Concision**: High signal-to-noise ratio
4. **Maintainability**: Easy to modify correctly
5. **Consistency**: Consistent with surrounding codebase

### Least Mechanism
Prefer standard tools over sophisticated machinery:
1. First: core language constructs (channels, slices, maps, loops, structs)
2. Then: standard library
3. Finally: external dependencies (only if necessary)

## Formatting

- Use `gofmt` (or `go fmt`) for all formatting—do not manually format
- Use **tabs for indentation**, spaces only when necessary
- No line length limit; break long expressions with trailing commas or parentheses; let `gofmt` handle alignment
- No parentheses around `if`, `for`, `switch` conditions

## Commentary

- Line comments `//` are the norm; block comments `/* */` are rare (mainly for disabling code)
- Doc comments appear before declarations with no blank line
- Doc comments should **start with the name** of the thing being documented:
```go
// Package regexp implements regular expression search.
package regexp

// Compile parses a regular expression and returns a Regexp.
func Compile(expr string) (*Regexp, error)
```

### Comment Sentences
- Doc comments should be complete sentences (capitalized, punctuated)
- End-of-line comments can be sentence fragments
- Comments explain **why**, not **what** (let code speak for itself)

### Package Comments
- Package comments usually start with `// Package <name> ...`
- For long package docs, use a dedicated `doc.go` file
- Document intended usage with runnable examples when possible

## Imports

- Group imports: standard library, then third-party, then local packages (blank line between groups)
- Use `goimports` to auto-format and manage imports
- Avoid `import .` except in rare test cases
- Blank imports (`import _ "pkg"`) only for side effects

## Semicolons

Go's lexer auto-inserts semicolons after tokens that could end a statement:
- Identifiers, literals, `break`, `continue`, `fallthrough`, `return`, `++`, `--`, `)`, `}`
- **Consequence**: Opening brace `{` must be on the same line as `if`, `for`, `switch`, `func`

## Naming Conventions

### Packages
- Short, lowercase, single-word names (no underscores or mixedCaps)
- Package name is base of source directory: `encoding/base64` → package `base64`
- Avoid stuttering: use `bufio.Reader`, not `bufio.BufReader`
- Constructor for sole exported type can be `New()`: `ring.New()`

### Exported Names
- First letter uppercase = exported (visible outside package)
- First letter lowercase = unexported (private to package)

### Getters/Setters
- Getter: `Owner()`, not `GetOwner()`
- Setter: `SetOwner()`

### Interfaces
- One-method interfaces: method name + `-er` suffix: `Reader`, `Writer`, `Stringer`
- Implement standard signatures when applicable (`Read`, `Write`, `String`, `Close`)

### Multi-word Names
- Use `MixedCaps` or `mixedCaps`, never underscores

### Receiver Names
- Short (one or two letters), abbreviation of the type
- Consistent across all methods for that type:
```go
func (b *Buffer) Read(p []byte) (n int, err error)  // not "buf" or "this"
func (b *Buffer) String() string
```

### Variable Names
- Length proportional to scope size; shorter names for smaller scopes
- Single-letter names OK for loop indices (`i`, `j`), readers (`r`), writers (`w`)
- Omit types from names: `users` not `userSlice`, `count` not `numUsers`
- Be specific when disambiguating: `userCount` vs `projectCount`

| Repetitive | Better |
|------------|--------|
| `var numUsers int` | `var users int` |
| `var nameString string` | `var name string` |
| `var primaryProject *Project` | `var primary *Project` |

### Constant Names
- Use `MixedCaps` like all other names—never `ALL_CAPS`
- Name constants based on their role, not their values:
```go
// Good
const MaxRetries = 3

// Bad
const THREE = 3
```

## Control Structures

### Braces
Opening brace must be on same line (semicolon insertion rule):
```go
// CORRECT
if condition {
    // ...
}

// WRONG - will not compile
if condition
{
}
```

### If Statements
- Omit `else` when body ends in `break`, `continue`, `goto`, or `return`
- Use initialization statements: `if err := doThing(); err != nil { return err }`
- Early returns for error handling—happy path flows down the page

### For Loops
```go
for init; condition; post { }   // C-style for
for condition { }               // while
for { }                         // infinite loop
for key, value := range m { }   // range over map/slice/string
for _, v := range slice { }     // ignore index with blank identifier
for k := range m { }            // keys only
```

### Switch
- No automatic fallthrough (use `fallthrough` keyword if needed)
- Cases can be comma-separated: `case 'a', 'b', 'c':`
- Expression-less switch is idiomatic for if-else chains:
```go
switch {
case x < 0: return -1
case x > 0: return 1
default:    return 0
}
```

### Type Switch
```go
switch v := x.(type) {
case int:    // v is int
case string: // v is string
default:     // v is same type as x
}
```

## Functions

### Multiple Return Values
- Return value AND error: `func Open(name string) (*File, error)`
- Return error as last value by convention

### Named Return Values
- Self-documenting: `func nextInt(b []byte, pos int) (value, nextPos int)`
- Bare `return` returns current values of named results (use sparingly)

### Defer
- Executes when surrounding function returns (LIFO order)
- Arguments evaluated when `defer` executes, not when deferred function runs
- Use for cleanup: `defer f.Close()`, `defer mu.Unlock()`

## Methods

### Pointers vs Values
- Value methods can be invoked on pointers and values
- Pointer methods can only be invoked on pointers
- Use pointer receiver when method modifies the receiver
- Use pointer receiver for large structs (avoid copy)
```go
func (s *MyStruct) SetValue(v int) { s.value = v }  // modifies receiver
func (s MyStruct) GetValue() int { return s.value } // doesn't modify
```

### Receiver Type Guidelines
- **Must use pointer**: method modifies receiver, receiver has `sync.Mutex` or similar
- **Prefer pointer**: large struct/array, or when unsure how code will grow
- **Prefer value**: small structs, built-in types (int, string), maps, functions, channels
- Keep methods for a type consistently all pointer or all value when possible

## Data Structures

### new vs make
- `new(T)` → allocates zeroed memory, returns `*T`
- `make(T, args)` → initializes slices, maps, channels; returns `T` (not pointer)

```go
p := new([]int)        // *[]int pointing to nil slice
v := make([]int, 10)   // []int with len=10, cap=10
m := make(map[string]int)
ch := make(chan int, 100)  // buffered channel
```

### Composite Literals
```go
return &File{fd: fd, name: name}  // labeled fields, zero values for omitted
a := []int{1, 2, 3}
m := map[string]int{"a": 1, "b": 2}
```

### Arrays
- Arrays are **values**; assigning copies all elements
- Passing to function copies the array (use slice or pointer for efficiency)
- Size is part of the type: `[10]int` and `[20]int` are distinct types
```go
array := [...]float64{7.0, 8.5, 9.1}  // compiler counts elements
x := Sum(&array)                       // pass pointer to avoid copy
```

### Slices
- Slices reference underlying arrays; assigning shares data
- Use `append(slice, elems...)` to grow
- `copy(dst, src)` copies elements

### Two-Dimensional Slices
```go
// Allocate each row independently (rows can grow/shrink)
picture := make([][]uint8, YSize)
for i := range picture {
    picture[i] = make([]uint8, XSize)
}

// Single allocation, sliced into rows (more efficient, fixed size)
picture := make([][]uint8, YSize)
pixels := make([]uint8, XSize*YSize)
for i := range picture {
    picture[i], pixels = pixels[:XSize], pixels[XSize:]
}
```

### Maps
- Zero value is `nil`; must initialize before use
- Comma-ok idiom: `v, ok := m[key]`
- Delete: `delete(m, key)` (safe even if key absent)

### Printing
```go
fmt.Printf("%v\n", value)   // default format
fmt.Printf("%+v\n", s)      // struct with field names
fmt.Printf("%#v\n", s)      // full Go syntax
fmt.Printf("%T\n", value)   // type of value
fmt.Printf("%q\n", str)     // quoted string
fmt.Printf("%x\n", bytes)   // hex encoding
```

Custom formatting via `String()` method:
```go
func (t *T) String() string {
    return fmt.Sprintf("%d/%g/%q", t.a, t.b, t.c)
}
```

## Interfaces

- Types implicitly implement interfaces—no `implements` keyword
- Keep interfaces small (one or two methods)
- Accept interfaces, return concrete types
- Empty interface `interface{}` (or `any`) holds any value

### Generality
- If a type exists only to implement an interface, export only the interface
- Constructor should return interface type, not implementing type:
```go
// Returns hash.Hash32, not *crc32.crc32
func NewIEEE() hash.Hash32
```

### Type Assertions
```go
s := val.(string)           // panics if not string
s, ok := val.(string)       // safe; ok=false if not string
```

### Interface Embedding
```go
type ReadWriter interface {
    Reader
    Writer
}
```

### Struct Embedding
```go
type Job struct {
    Command string
    *log.Logger  // embedded: Job gains Logger's methods
}
job.Println("...")  // calls embedded Logger's method
```

## Initialization

### Constants
```go
type ByteSize float64

const (
    _           = iota // ignore first value by assigning to blank identifier
    KB ByteSize = 1 << (10 * iota)
    MB
    GB
    TB
)
```

### Variables
- Can be initialized with expressions computed at run time
```go
var (
    home   = os.Getenv("HOME")
    user   = os.Getenv("USER")
)
```

### The init Function
- Each source file can have multiple `init()` functions
- Called after all variable declarations are evaluated
- Called after all imported packages are initialized
```go
func init() {
    if user == "" {
        log.Fatal("$USER not set")
    }
}
```

## The Blank Identifier

- `_` discards values: `_, err := f.Read(buf)`
- Import for side effects: `import _ "net/http/pprof"`
- Compile-time interface check: `var _ json.Marshaler = (*T)(nil)`

## Error Handling

### Conventions
- Return `error` as last return value
- `nil` error = success
- Error strings: lowercase, no punctuation at end
- Prefix with package/operation: `"image: unknown format"`

### Pattern
```go
f, err := os.Open(name)
if err != nil {
    return err  // or wrap: fmt.Errorf("open config: %w", err)
}
defer f.Close()
```

### Indent Error Flow
Handle errors before proceeding—happy path flows down the page:
```go
// Good: error handled first, success flows down
if err := doSomething(); err != nil {
    return err
}
// continue with success case...
```

### Error Wrapping
- Use `%w` when callers need to inspect the underlying error with `errors.Is`/`errors.As`
- Use `%v` when the underlying error is an implementation detail
- Use `fmt.Errorf` for formatting, not `errors.New(fmt.Sprintf(...))`
- Add context without duplicating information the error already contains:
```go
// Good: adds context without duplicating path info from os.Open
return fmt.Errorf("loading config: %w", err)

// Avoid when underlying error already includes the path
return fmt.Errorf("failed to open %s: %w", path, err)
```

### Custom Errors
```go
type PathError struct {
    Op   string
    Path string
    Err  error
}

func (e *PathError) Error() string {
    return e.Op + " " + e.Path + ": " + e.Err.Error()
}
```

### Panic and Recover
- `panic`: only for truly unrecoverable errors (prefer returning errors)
- `recover`: only useful inside deferred functions
- Avoid panics for expected errors; if recovering, do it at well-defined boundaries (e.g., top of goroutine/request handler)

### Must Functions
- Functions that panic on error use `MustXYZ` naming convention
- Only use during initialization or in tests, not for user input:
```go
var re = regexp.MustCompile(`^\d+$`)  // OK: initialization
```

## Context

- Pass `context.Context` as the first parameter to functions that need it
- Do not store contexts in structs; pass them explicitly
- Do not create custom context types—use only `context.Context`
- Name it `ctx`; never pass `nil` context (use `context.TODO()` if unsure)
- Derive with `context.WithCancel/Timeout`; always `defer cancel()`
- Functions taking a context should return an error (context may be cancelled)

## Concurrency

### Goroutines
```go
go func() {
    // runs concurrently
}()
```

### Channels
```go
ch := make(chan int)      // unbuffered (synchronous)
ch := make(chan int, 10)  // buffered

ch <- v    // send
v := <-ch  // receive (blocks until data available)
close(ch)  // close channel

// Range over channel
for v := range ch {
    // receives until ch is closed
}
```

### Select
```go
select {
case v := <-ch1:
    // received from ch1
case ch2 <- x:
    // sent to ch2
default:
    // no channel ready (non-blocking)
}
```

### Philosophy
- "Do not communicate by sharing memory; share memory by communicating"
- Choose the simplest synchronization: channels for communication/coordination, mutexes for shared mutable state
- Avoid goroutine leaks: ensure goroutines can exit (context cancellation, closing channels)
- Only the sender should close a channel, never the receiver

### Channels of Channels
- Channels are first-class values; can be passed around
- Use reply channels for request/response patterns:
```go
type Request struct {
    args       []int
    resultChan chan int
}

// Client sends request with reply channel
req := &Request{[]int{3, 4, 5}, make(chan int)}
requests <- req
result := <-req.resultChan

// Server sends result back on reply channel
func handle(queue chan *Request) {
    for req := range queue {
        req.resultChan <- process(req.args)
    }
}
```

### Parallelization
```go
var numCPU = runtime.NumCPU()           // number of hardware cores
// or: runtime.GOMAXPROCS(0)            // user-specified limit

func (v Vector) DoAll(u Vector) {
    c := make(chan int, numCPU)
    for i := 0; i < numCPU; i++ {
        go v.DoSome(i*len(v)/numCPU, (i+1)*len(v)/numCPU, u, c)
    }
    for i := 0; i < numCPU; i++ {
        <-c  // wait for all goroutines
    }
}
```

## Generics

- Use generics only when they fulfill clear business requirements
- Prefer conventional approaches (slices, maps, interfaces) when they work well
- Do not use generics just because an algorithm doesn't care about element types
- If only one type is instantiated in practice, start without generics

## Testing

### Test Failures
Tests should fail with helpful messages detailing:
- What caused the failure
- What inputs resulted in an error
- The actual result vs. what was expected

### Table-Driven Tests
Use table-driven tests to reduce repetition and clarify test cases:
```go
tests := []struct {
    name    string
    input   string
    want    int
    wantErr bool
}{
    {"valid", "42", 42, false},
    {"invalid", "abc", 0, true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := Parse(tt.input)
        if (err != nil) != tt.wantErr {
            t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
        }
        if got != tt.want {
            t.Errorf("Parse() = %v, want %v", got, tt.want)
        }
    })
}
```

### Test Helpers
- Use `t.Helper()` in helper functions so failures report the caller's line
- Use `t.Fatalf` when continuing doesn't make sense
- Place test fixtures in `testdata/` directory

## Common Patterns

### Interface Compliance Check
```go
var _ io.Reader = (*MyType)(nil)  // compile error if MyType doesn't implement Reader
```

### Functional Options
```go
func NewServer(addr string, opts ...Option) *Server
```

### Constructor
```go
func NewThing(cfg Config) (*Thing, error) {
    // validate, initialize, return
}
```

## Build & Test Commands

```bash
go build ./...           # build all packages
go test ./...            # test all packages
go test -race ./...      # test with race detector
go vet ./...             # static analysis
go fmt ./...             # format all files
golangci-lint run        # comprehensive linting (if available)
```
