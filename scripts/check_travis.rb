#!/usr/bin/env ruby

require 'net/http'
require 'json'
require 'optparse'
require 'openssl'

class String
  COLORS = { yellow: "\e[0;33m", green: "\e[0;32m", red: "\e[0;31m", blue: "\e[0;34m", reset: "\e[0m" }.freeze

  def self.color=(color); @color = color; end
  def self.color?; !!@color; end

  def color(color)
    String.color? ? "#{COLORS[color]}#{self}#{COLORS[:reset]}" : self
  end
end

class TravisBuild
  TRAVIS_URL = "https://api.travis-ci.org/repos/%s/builds".freeze

  def initialize(shas, color = true)
    @shas = shas
    remote = `git remote -v`.match(/github.com[\/:]([^\s]+?)(.git| )/)
    @repo = remote && remote[1]
    String.color = color
  end

  def log_with_travis_status
    output = [travis? ? "https://travis-ci.org/#{@repo}".color(:yellow) : "No Travis".color(:blue)]

    `git log #{@shas} --oneline`.split("\n").each do |commit|
      sha, message = commit.split(" ", 2)
      output << "\n#{sha.color(:yellow)} #{message}#{travis_status(sha) if travis?}"
    end

    puts output.join
  end

  def log_sorted_by_author_with_travis_status
    output = ["Bump #{@repo}:\n".color(:blue)]

    commits_by_author = Hash.new("")
    `git log #{@shas} --pretty=format:'%h %an: %s'`.split("\n").each do |commit|
      sha, message = commit.split(" ", 2)
      author, message = message.split(":", 2)
      commits_by_author[author] += "\n    #{message}#{travis_status(sha) if travis?}"
    end

    commits_by_author.each { |author, commit| output << "  #{author.color(:yellow)}:#{commit}\n" }
    puts output.join
  end


  def travis_json
    @travis_json ||= begin
      uri = URI(TRAVIS_URL % @repo)
      http = Net::HTTP.new(uri.host, uri.port)
      http.use_ssl = true
      http.verify_mode = OpenSSL::SSL::VERIFY_NONE
      JSON.parse(http.request(Net::HTTP::Get.new(uri.path)).body)
    end
  end

  private

  def travis?
    @travis ||= @repo && travis_json && travis_json.any?
  end

  def travis_status(sha)
    build = travis_json.find { |build| build["commit"].start_with?(sha) }

    message = case build && build["result"]
      when 0; "Travis Success: #{build_url(build)}".color(:green)
      when 1; "Travis Failed: #{build_url(build)}".color(:red)
      else; "Travis #{build ? "#{build["state"]}: #{build_url(build)}" : "Unknown"}".color(:blue)
    end

    " ( #{message} )"
  end

  def build_url(build)
    "https://travis-ci.org/#{@repo}/builds/#{build["id"]}"
  end
end

if __FILE__ == $PROGRAM_NAME
  travis = TravisBuild.new(ARGV[0], true)
  travis.log_with_travis_status
end
