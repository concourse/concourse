#!/usr/bin/env ruby

require 'json'
require 'net/http'
require 'optparse'
require 'uri'

class CheckSection
  def initialize(name)
    @name = name
    @checks = []
  end

  def to_s
    check = checks.map(&:to_s).join("\n")
    "#- #{name}\n#{check}"
  end

  def <<(check)
    checks << check
  end

  private

  attr_reader :name, :checks
end

class Check
  def initialize(name, concourse)
    @name = name
    @concourse = concourse
  end

  def to_s
    "#{name}: concourse.check #{concourse.url} #{concourse.username} #{concourse.password} #{name}"
  end

  private

  attr_reader :name, :concourse
end

class ConcourseGroup
  def initialize(attrs)
    @attrs = attrs
  end

  def name
    attrs.fetch('name')
  end

  def job_names
    attrs.fetch('jobs')
  end

  private

  attr_reader :attrs
end

class Concourse
  attr_reader :url, :username, :password

  def initialize(url, username, password)
    @url = url
    @username = username
    @password = password
  end

  def groups
    pipeline.fetch('groups').map { |g| ConcourseGroup.new(g) }
  end

  def all_jobs
    pipeline.fetch('jobs').map { |j| j.fetch('name') }
  end

  def orphaned_jobs
    grouped_jobs = groups.flat_map(&:job_names)

    all_jobs - grouped_jobs
  end

  private

  def pipeline
    @pipeline if @pipeline

    req = Net::HTTP::Get.new(url.to_s + "/api/v1/config")
    req.basic_auth(username, password)

    res = Net::HTTP.start(url.hostname, url.port) do |http|
      http.request(req)
    end

    @pipeline = JSON.load(res.body)
  end
end

class Checkfile
  def self.build_from(concourse)
    Checkfile.new(concourse)
  end

  def initialize(concourse)
    @concourse = concourse
  end

  def to_s
    sections.map do |section|
      section.to_s
    end.join("\n\n")
  end

  private

  attr_reader :concourse

  def sections
    group_sections + misc_sections
  end

  def group_sections
    concourse.groups.map do |group|
      section = CheckSection.new(group.name)

      group.job_names.each do |job_name|
        section << Check.new(job_name, concourse)
      end

      section
    end
  end

  def misc_sections
    orphaned_jobs = concourse.orphaned_jobs
    return [] if orphaned_jobs.empty?

    misc = CheckSection.new('misc')
    orphaned_jobs.each do |job_name|
      misc << Check.new(job_name, concourse)
    end

    [misc]
  end
end

options = {}

OptionParser.new do |opts|
  opts.banner = "usage: generate_checkman.rb [options]"

  opts.on("-u", "--url URL", "url of concourse server") do |url|
    options[:url] = url
  end

  opts.on("-n", "--username USERNAME", "username of concourse server") do |username|
    options[:username] = username
  end

  opts.on("-p", "--password PASSWORD", "password of concourse server") do |password|
    options[:password] = password
  end
end.parse!

url = options.fetch(:url) { warn "missing --url"; exit 1 }
username = options.fetch(:username) { warn "missing --username"; exit 1 }
password = options.fetch(:password) { warn "missing --password"; exit 1 }

concourse = Concourse.new(URI(url), username, password)
puts Checkfile.build_from(concourse)
