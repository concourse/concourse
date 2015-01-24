#!/usr/bin/env ruby

require 'yaml'

pipeline_path = ARGV[0]
url = ARGV[1]
username = ARGV[2]
password = ARGV[3]

if ![pipeline_path, url, username, password].all?
  warn 'missing args'
  exit 1
end

pipeline = YAML.load_file(pipeline_path)

seen_jobs = []

pipeline.fetch('groups').each do |group|
  group_name = group.fetch('name')
  group_jobs = group.fetch('jobs')

  puts "#- #{group_name}"
  group_jobs.each do |job_name|
    seen_jobs << job_name
    puts "#{job_name}: concourse.check #{url} #{username} #{password} #{job_name}"
  end
  puts
end

all_jobs = pipeline.fetch('jobs').map { |j| j.fetch('name') }
no_group_jobs = all_jobs - seen_jobs
exit if no_group_jobs.empty?

puts '#- misc'
no_group_jobs.each do |job_name|
  puts "#{job_name}: concourse.check #{url} #{username} #{password} #{job_name}"
end
