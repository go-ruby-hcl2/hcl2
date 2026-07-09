# frozen_string_literal: true
#
# Pure-Ruby usage of the HCL2 module, as provided by go-embedded-ruby (rbgo).
# Run it with:  rbgo examples/hcl2_usage.rb

require "hcl2"

# HCL2.parse reads a whole document into an ordered Ruby Hash. Attributes keep
# their native types (String / Integer / Boolean / Array) and blocks nest.
doc = <<~HCL
  name    = "dimail"
  version = 3
  ports   = [80, 443]

  server "web" {
    region = "eu-west"
  }
HCL
cfg = HCL2.parse(doc)
puts cfg["name"]              # => dimail
p cfg["ports"]                # => [80, 443]
p cfg["server"]               # => {"web" => {"region" => "eu-west"}}

# HCL2.eval supplies variables so ${...} interpolations resolve.
p HCL2.eval('greeting = "hello, ${who}"', variables: { who: "world" })

# HCL2.eval_expr evaluates a single expression, including built-in functions.
p HCL2.eval_expr("2 + 3 * 4")   # => 14
p HCL2.eval_expr('upper("go")') # => "GO"
p HCL2.eval_expr("max(1, 7, 3)") # => 7

# A malformed document raises HCL2::Error.
begin
  HCL2.parse("name =")
rescue HCL2::Error => e
  puts "rescued #{e.class}"
end
