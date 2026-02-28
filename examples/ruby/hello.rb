# verify ruby version
raise "need ruby 3+, got #{RUBY_VERSION}" unless RUBY_VERSION >= "3"
puts "ruby #{RUBY_VERSION}"

# stdlib json
require 'json'
data = { status: "ok", sandbox: true }
raise "json round-trip failed" unless JSON.parse(JSON.dump(data))["status"] == "ok"
puts "json: ok"

# stdlib net/http import (don't open a socket)
require 'net/http'
puts "net/http: ok"

puts "all checks passed"
