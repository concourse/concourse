module Concourse.BuildResources exposing (empty, fetch)

import Http
import Task exposing (Task)

import Concourse

empty : Concourse.BuildResources
empty =
  { inputs = []
  , outputs = []
  }

fetch : Concourse.BuildId -> Task Http.Error Concourse.BuildResources
fetch buildId =
  Http.get Concourse.decodeBuildResources ("/api/v1/builds/" ++ toString buildId ++ "/resources")
