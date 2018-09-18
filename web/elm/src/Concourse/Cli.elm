module Concourse.Cli exposing (downloadUrl)


downloadUrl : String -> String -> String
downloadUrl arch platform =
    "/api/v1/cli?arch=" ++ arch ++ "&platform=" ++ platform
