# Docstrings and code comments

Docstrings document the contract: what it does, parameters with types and units, return value, errors raised, side effects, and thread-safety or idempotency when it matters. Add one example for non-obvious use, and test it where the language allows.

Comments explain why: hidden constraints, workarounds with an issue link, invariants, surprising behavior, performance trade-offs. Delete comments that narrate the code. A good name can often replace a comment.

Do not restate the signature. "Gets the user. Takes an id and returns a user." adds nothing. Start function comments with a verb stating what the function does and returns, so a reader can scan a file. Class comments are noun phrases. Rationale for a design belongs in an inline comment or design doc, not in a method's contract.

Follow the format already in the file, then the project's convention, then this table. Do not change source comments in a documentation task without approval, and do not reformat existing docstrings as a side effect.

| Language | Format | Generator or check |
|---|---|---|
| Python | PEP 257 plus Google or NumPy sections | Sphinx, pdoc; `interrogate` for coverage; `doctest` runs examples |
| TypeScript | TSDoc | TypeDoc |
| JavaScript | JSDoc | JSDoc |
| Java | Javadoc | `javadoc` |
| Kotlin | KDoc | Dokka |
| C# | XML doc comments (`///`) | DocFX |
| Go | Comment starting with the identifier's name | pkgsite; `Example` functions run as tests |
| Rust | `///`, with `# Examples`, `# Errors`, `# Panics` | `cargo doc`; `cargo test --doc` runs examples |

Python, Google style:

```python
def retry(fn, attempts=3, base_delay_s=0.2):
    """Call fn, retrying on TimeoutError with exponential backoff.

    Args:
        fn: Zero-argument callable.
        attempts: Total tries including the first. Must be at least 1.
        base_delay_s: Delay before the second try; doubles each time.

    Returns:
        The return value of fn.

    Raises:
        TimeoutError: If every attempt times out.
    """
```

C#:

```csharp
/// <summary>Reserves stock for an order, or returns false if any line is short.</summary>
/// <param name="orderId">The order to reserve for. Must exist.</param>
/// <returns>True when every line was reserved.</returns>
/// <exception cref="OrderNotFoundException">No order has this ID.</exception>
```

Go:

```go
// Retry calls fn up to attempts times, doubling baseDelay after each timeout.
// It returns the last error when every attempt fails.
func Retry(fn func() error, attempts int, baseDelay time.Duration) error
```
