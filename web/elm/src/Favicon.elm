module Favicon exposing (set)

import Concourse
import Concourse.BuildStatus
import Native.Favicon
import Task exposing (Task)


set : Maybe Concourse.BuildStatus -> Task x ()
set status =
    let
        iconName =
            case status of
                Just status ->
                    "/public/images/favicon-" ++ Concourse.BuildStatus.show status ++ ".png"

                Nothing ->
                    "/public/images/favicon.png"
    in
    Native.Favicon.set iconName
