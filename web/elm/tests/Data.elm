module Data exposing
    ( check
    , jobBuild
    , jobId
    , pipelineName
    , resource
    , resourceName
    , teamName
    , version
    , versionedResource
    )

import Concourse
import Concourse.BuildStatus as BuildStatus
import Dict exposing (Dict)
import Time


check : Concourse.CheckStatus -> Concourse.Check
check status =
    case status of
        Concourse.Started ->
            { id = 0
            , status = Concourse.Started
            , createTime = Just <| Time.millisToPosix 0
            , startTime = Just <| Time.millisToPosix 0
            , endTime = Nothing
            , checkError = Nothing
            }

        Concourse.Succeeded ->
            { id = 0
            , status = Concourse.Succeeded
            , createTime = Just <| Time.millisToPosix 0
            , startTime = Just <| Time.millisToPosix 0
            , endTime = Just <| Time.millisToPosix 1000
            , checkError = Nothing
            }

        Concourse.Errored ->
            { id = 0
            , status = Concourse.Errored
            , createTime = Just <| Time.millisToPosix 0
            , startTime = Just <| Time.millisToPosix 0
            , endTime = Just <| Time.millisToPosix 1000
            , checkError = Just "something broke"
            }


resource : String -> Concourse.Resource
resource pinnedVersion =
    { teamName = teamName
    , pipelineName = pipelineName
    , name = resourceName
    , failingToCheck = False
    , checkError = ""
    , checkSetupError = ""
    , lastChecked = Nothing
    , pinnedVersion = Just <| version pinnedVersion
    , pinnedInConfig = False
    , pinComment = Nothing
    , icon = Nothing
    }


teamName =
    "team"


pipelineName =
    "pipeline"


resourceName =
    "resource"


versionedResource : String -> Int -> Concourse.VersionedResource
versionedResource v id =
    { id = id
    , version = version v
    , metadata = []
    , enabled = True
    }


version : String -> Dict String String
version v =
    Dict.fromList [ ( "version", v ) ]


jobId : Concourse.JobIdentifier
jobId =
    { teamName = "t"
    , pipelineName = "p"
    , jobName = "j"
    }


jobBuild : BuildStatus.BuildStatus -> Concourse.Build
jobBuild status =
    { id = 1
    , name = "1"
    , job = Just jobId
    , status = status
    , duration =
        { startedAt =
            case status of
                BuildStatus.BuildStatusPending ->
                    Nothing

                _ ->
                    Just <| Time.millisToPosix 0
        , finishedAt =
            if BuildStatus.isRunning status then
                Nothing

            else
                Just <| Time.millisToPosix 0
        }
    , reapTime = Nothing
    }
