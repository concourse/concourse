Gem::Specification.new do |s|
  s.name     = "concourse-docs"
  s.version  = "0.0.0"
  s.authors  = ["Alex Suraci"]
  s.email    = ["suraci.alex@gmail.com"]

  s.summary = "helpers for our docs"

  s.files         = Dir["{lib,bin,public}/**/*"]
  s.require_paths = ["lib"]

  s.license = "Apache-2.0"

  s.add_runtime_dependency "anatomy", "~> 0.4.0"
end
