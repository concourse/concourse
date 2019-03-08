module Network.BuildPlan exposing (fetch)

import Concourse
import Http
import Task exposing (Task)


fetch : Concourse.BuildId -> Task Http.Error Concourse.BuildPlan
fetch buildId =
    Http.toTask <|
        Http.get ("/api/v1/builds/" ++ toString buildId ++ "/plan") Concourse.decodeBuildPlan
