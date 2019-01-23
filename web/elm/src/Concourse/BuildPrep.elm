module Concourse.BuildPrep exposing (fetch)

import Concourse
import Http
import Task exposing (Task)


fetch : Concourse.BuildId -> Task Http.Error Concourse.BuildPrep
fetch buildId =
    Http.toTask
        << flip Http.get Concourse.decodeBuildPrep
    <|
        "/api/v1/builds/"
            ++ toString buildId
            ++ "/preparation"
