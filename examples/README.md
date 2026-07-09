# Ruby examples

Pure-Ruby examples for the `hcl2` library as provided by
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby) (rbgo). Run them
with the `rbgo` interpreter:

```sh
rbgo examples/hcl2_usage.rb
```

| File | Shows |
| --- | --- |
| [`hcl2_usage.rb`](hcl2_usage.rb) | Parsing a document, evaluating with variables, single-expression + built-in functions, and HCL2::Error handling. |

Each example is executed as-is under rbgo (`require "hcl2"`).
