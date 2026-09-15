#!/usr/bin/env bash
set -euo pipefail

ruby <<'RUBY'
require "yaml"

weekly_path = ".github/workflows/weekly-report.yml"
weekly = YAML.load_file(weekly_path)
weekly_trigger = weekly["on"] || weekly[true]
weekly_dispatch = weekly_trigger["workflow_dispatch"]
abort "weekly workflow dispatch is missing" unless weekly_dispatch
weekly_guard = weekly.fetch("jobs").fetch("report").fetch("if").to_s
abort "weekly workflow lacks main ref guard" unless weekly_guard.include?("github.ref == 'refs/heads/main'")
checkout = weekly.fetch("jobs").fetch("report").fetch("steps").find { |step| step["uses"] == "actions/checkout@v4" }
abort "weekly workflow does not pin checkout to main" unless checkout.dig("with", "ref") == "refs/heads/main"

cd_path = ".github/workflows/cd.yml"
cd = YAML.load_file(cd_path)
cd_trigger = cd["on"] || cd[true]
cd_dispatch = cd_trigger["workflow_dispatch"]
abort "CD workflow dispatch has an invalid configuration" if cd_dispatch.is_a?(Hash) && cd_dispatch.key?("branches")
%w[discover build-and-push deploy].each do |job_name|
  guard = cd.fetch("jobs").fetch(job_name).fetch("if").to_s
  abort "CD job #{job_name} lacks main ref guard" unless guard.include?("github.ref == 'refs/heads/main'")
end

ci = File.read(".github/workflows/ci.yml")
abort "CI does not run actionlint" unless ci.include?("raven-actions/actionlint@v2")
puts "workflow contract tests: all tests passed"
RUBY
