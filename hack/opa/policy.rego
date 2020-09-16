package concourse

default decision = {"allowed": true}

# uncomment to include deny rules
#decision = {"allowed": false, "reasons": reasons} {
#  count(deny) > 0
#  reasons := deny
#}

deny["cannot use docker-image types"] {
  input.action == "UseImage"
  input.data.image_type == "docker-image"
}

deny["cannot run privileged tasks"] {
  input.action == "SaveConfig"
  input.data.jobs[_].plan[_].privileged
}

deny["cannot use privileged resource types"] {
  input.action == "SaveConfig"
  input.data.resource_types[_].privileged
}
