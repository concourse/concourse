require 'open3'

module Fly
  class FlyError < RuntimeError; end

  def fly(command)
    run "fly -t testpilot #{command}"
  end

  def fly_fail(command)
    run "fly -t testpilot #{command}"
  rescue FlyError
    nil
  else
    raise "expected '#{command}' to not succeed"
  end

  def fly_login(team_name)
    fly("login -k -c #{ATC_URL} -n #{team_name}")
  end

  def fly_table(command)
    output = run "fly --print-table-headers -t testpilot #{command}"

    rows = []
    headers = nil
    output.each_line do |line|
      cols = line.strip.split(/\s{2,}/)

      if headers
        row = {}
        headers.each.with_index do |key, i|
          row[key] = cols[i]
        end

        rows << row
      else
        headers = cols
      end
    end

    rows
  end

  def fly_with_input(command, input)
    run "echo '#{input}' | fly -t testpilot #{command}"
  end

  def generate_team_name
    "wats-team-#{SecureRandom.uuid}"
  end

  def cleanup_teams
    fly_table('teams').each do |team|
      name = team['name']

      if name.match?(/^wats-team-/)
        fly_with_input("destroy-team -n #{name}", name)
      end
    end
  end

  private

  def run(command)
    output, status = Open3.capture2e command

    raise FlyError, "'#{command}' failed (status #{status.exitstatus}):\n\n#{output}" \
      unless status.success?

    output
  end
end
