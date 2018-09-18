module Concourse.BuildPrep exposing (..)

import Http
import Task exposing (Task)
import Concourse


fetch : Concourse.BuildId -> Task Http.Error Concourse.BuildPrep
fetch buildId =
    Http.toTask
        << flip Http.get Concourse.decodeBuildPrep
    <|
        "/api/v1/builds/"
            ++ toString buildId
            ++ "/preparation"
