module Concourse.BuildResources exposing (empty, fetch)

import Concourse
import Http
import Task exposing (Task)


empty : Concourse.BuildResources
empty =
    { inputs = []
    , outputs = []
    }


fetch : Concourse.BuildId -> Task Http.Error Concourse.BuildResources
fetch buildId =
    Http.toTask
        << Http.get ("/api/v1/builds/" ++ toString buildId ++ "/resources")
    <|
        Concourse.decodeBuildResources
