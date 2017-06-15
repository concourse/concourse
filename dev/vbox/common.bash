function abort() {
  echo $'\e[31m'"$@"$'\e[0m' >&2
  exit 1
}

ran_by="false"

function by() {
  if [ "$ran_by" = "true" ]; then
    echo ""
  fi

  ran_by="true"

  echo $'\e[1m'"$@"$'\e[0m'
}

