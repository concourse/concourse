require 'open3'

module Fly
  class FlyError < RuntimeError; end

  def target
    team_name
  end

  def fly(command)
    run "fly -t #{target} #{command}"
  end

  def fly_fail(command)
    run "fly -t #{target} #{command}"
  rescue FlyError
    nil
  else
    raise "expected '#{command}' to not succeed"
  end

  def fly_login(team_name)
    fly("login -k -c #{ATC_URL} -n #{team_name} -u #{ATC_USERNAME} -p #{ATC_PASSWORD}")
  end

  def fly_with_input(command, input)
    run "echo '#{input}' | fly -t #{target} #{command}"
  end

  def generate_team_name
    "wats-team-#{SecureRandom.uuid}"
  end

  private

  def run(command)
    output, status = Open3.capture2e({ 'HOME' => fly_home }, command)

    raise FlyError, "'#{command}' failed (status #{status.exitstatus}):\n\n#{output}" \
      unless status.success?

    output
  end
end
