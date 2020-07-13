module Data exposing
    ( check
    , dashboardPipeline
    , elementPosition
    , httpInternalServerError
    , httpNotFound
    , httpNotImplemented
    , httpUnauthorized
    , job
    , jobBuild
    , jobId
    , jobName
    , leftClickEvent
    , pipeline
    , pipelineName
    , resource
    , resourceName
    , teamName
    , version
    , versionedResource
    , withArchived
    , withGroups
    , withName
    , withPaused
    , withPublic
    )

import Browser.Dom
import Concourse
import Concourse.BuildStatus as BuildStatus
import Dashboard.Group.Models
import Dict exposing (Dict)
import Http
import Json.Encode
import Test.Html.Event as Event
import Time


httpUnauthorized : Result Http.Error a
httpUnauthorized =
    Err <|
        Http.BadStatus
            { url = "http://example.com"
            , status =
                { code = 401
                , message = ""
                }
            , headers = Dict.empty
            , body = ""
            }


httpNotFound : Result Http.Error a
httpNotFound =
    Err <|
        Http.BadStatus
            { url = "http://example.com"
            , status =
                { code = 404
                , message = "not found"
                }
            , headers = Dict.empty
            , body = ""
            }


httpNotImplemented : Result Http.Error a
httpNotImplemented =
    Err <|
        Http.BadStatus
            { url = "http://example.com"
            , status =
                { code = 501
                , message = "not implemented"
                }
            , headers = Dict.empty
            , body = ""
            }


httpInternalServerError : Result Http.Error a
httpInternalServerError =
    Err <|
        Http.BadStatus
            { url = "http://example.com"
            , status =
                { code = 500
                , message = "internal server error"
                }
            , headers = Dict.empty
            , body = ""
            }


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


pipeline : String -> Int -> Concourse.Pipeline
pipeline team id =
    { id = id
    , name = "pipeline-" ++ String.fromInt id
    , paused = False
    , archived = False
    , public = True
    , teamName = team
    , groups = []
    }


dashboardPipeline : Int -> Bool -> Dashboard.Group.Models.Pipeline
dashboardPipeline id public =
    { id = id
    , name = pipelineName
    , teamName = teamName
    , public = public
    , isToggleLoading = False
    , isVisibilityLoading = False
    , paused = False
    , archived = False
    , stale = False
    , jobsDisabled = False
    , isFavorited = False
    }


withPaused : Bool -> { r | paused : Bool } -> { r | paused : Bool }
withPaused paused p =
    { p | paused = paused }


withArchived : Bool -> { r | archived : Bool } -> { r | archived : Bool }
withArchived archived p =
    { p | archived = archived }


withPublic : Bool -> { r | public : Bool } -> { r | public : Bool }
withPublic public p =
    { p | public = public }


withName : String -> { r | name : String } -> { r | name : String }
withName name p =
    { p | name = name }


withGroups : List Concourse.PipelineGroup -> { r | groups : List Concourse.PipelineGroup } -> { r | groups : List Concourse.PipelineGroup }
withGroups groups p =
    { p | groups = groups }


job : Int -> Concourse.Job
job pipelineID =
    { name = jobName
    , pipelineName = "pipeline-" ++ String.fromInt pipelineID
    , teamName = teamName
    , nextBuild = Nothing
    , finishedBuild = Nothing
    , transitionBuild = Nothing
    , paused = False
    , disableManualTrigger = False
    , inputs = []
    , outputs = []
    , groups = []
    }


jobName =
    "job"


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


elementPosition : Browser.Dom.Element
elementPosition =
    { scene =
        { width = 0
        , height = 0
        }
    , viewport =
        { width = 0
        , height = 0
        , x = 0
        , y = 0
        }
    , element =
        { x = 0
        , y = 0
        , width = 1
        , height = 1
        }
    }


leftClickEvent : ( String, Json.Encode.Value )
leftClickEvent =
    Event.custom "click" <|
        Json.Encode.object
            [ ( "ctrlKey", Json.Encode.bool False )
            , ( "altKey", Json.Encode.bool False )
            , ( "metaKey", Json.Encode.bool False )
            , ( "shiftKey", Json.Encode.bool False )
            , ( "button", Json.Encode.int 0 )
            ]
