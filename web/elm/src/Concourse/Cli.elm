module Concourse.Cli exposing (downloadUrl, Cli(..))


downloadUrl : String -> String -> String
downloadUrl arch platform =
    "/api/v1/cli?arch=" ++ arch ++ "&platform=" ++ platform


type Cli
    = OSX
    | Windows
    | Linux
