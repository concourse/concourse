
SCRIPT=$(basename $0)
mkdir -p /var/vcap/sys/log/monit

exec 1>> /var/vcap/sys/log/monit/$SCRIPT.log
exec 2>> /var/vcap/sys/log/monit/$SCRIPT.err.log

pid_guard() {
  pidfile=$1
  name=$2

  if [ -f "$pidfile" ]; then
    pid=$(head -1 "$pidfile")

    if [ -n "$pid" ] && [ -e /proc/$pid ]; then
      echo "$name is already running, please stop it first"
      exit 1
    fi

    echo "Removing stale pidfile..."
    rm $pidfile
  fi
}

wait_pidfile() {
  pidfile=$1
  try_kill=$2
  timeout=${3:-0}
  force=${4:-0}
  countdown=$(( $timeout * 10 ))

  if [ -f "$pidfile" ]; then
    pid=$(head -1 "$pidfile")

    if [ -z "$pid" ]; then
      echo "Unable to get pid from $pidfile"
      exit 1
    fi

    if [ -e /proc/$pid ]; then
      if [ "$try_kill" = "1" ]; then
        echo "Killing $pidfile: $pid "
        kill $pid
      fi
      while [ -e /proc/$pid ]; do
        sleep 0.1
        [ "$countdown" != '0' -a $(( $countdown % 10 )) = '0' ] && echo -n .
        if [ $timeout -gt 0 ]; then
          if [ $countdown -eq 0 ]; then
            if [ "$force" = "1" ]; then
              echo -ne "\nKill timed out, using kill -9 on $pid... "
              kill -9 $pid
              sleep 0.5
            fi
            break
          else
            countdown=$(( $countdown - 1 ))
          fi
        fi
      done
      if [ -e /proc/$pid ]; then
        echo "Timed Out"
      else
        echo "Stopped"
      fi
    else
      echo "Process $pid is not running"
    fi

    rm -f $pidfile
  else
    echo "Pidfile $pidfile doesn't exist"
  fi
}

kill_and_wait() {
  pidfile=$1
  # Monit default timeout for start/stop is 30s
  # Append 'with timeout {n} seconds' to monit start/stop program configs
  timeout=${2:-25}
  force=${3:-1}

  wait_pidfile $pidfile 1 $timeout $force
}

check_mount() {
  opts=$1
  exports=$2
  mount_point=$3

  if grep -qs $mount_point /proc/mounts; then
    echo "Found NFS mount $mount_point"
  else
    echo "Mounting NFS..."
    mount $opts $exports $mount_point
    if [ $? != 0 ]; then
      echo "Cannot mount NFS from $exports to $mount_point, exiting..."
      exit 1
    fi
  fi
}

# Check the syntax of a sudoers file.
check_sudoers() {
  /usr/sbin/visudo -c -f "$1"
}

# Check the syntax of a sudoers file and if it's ok install it.
install_sudoers() {
  src="$1"
  dest="$2"

  check_sudoers "$src"

  if [ $? -eq 0 ]; then
    chown root:root "$src"
    chmod 0440 "$src"
    cp -p "$src" "$dest"
  else
    echo "Syntax error in sudoers file $src"
    exit 1
  fi
}

# Add a line to a file if it is not already there.
file_must_include() {
  file="$1"
  line="$2"

  # Protect against empty $file so it doesn't wait for input on stdin.
  if [ -n "$file" ]; then
    grep --quiet "$line" "$file" || echo "$line" >> "$file"
  else
    echo 'File name is required'
    exit 1
  fi
}
