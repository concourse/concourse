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
    , jobBuildId
    , jobId
    , jobName
    , leftClickEvent
    , longJobBuildId
    , pipeline
    , pipelineId
    , pipelineName
    , resource
    , resourceId
    , resourceName
    , resourceVersionId
    , shortJobId
    , shortResourceId
    , teamName
    , version
    , versionedResource
    , withArchived
    , withBackgroundImage
    , withBuildName
    , withCheckError
    , withDisableManualTrigger
    , withDuration
    , withFailingToCheck
    , withFinishedBuild
    , withGroups
    , withHovered
    , withIcon
    , withId
    , withInstanceVars
    , withJob
    , withJobName
    , withLastChecked
    , withName
    , withNextBuild
    , withPaused
    , withPinComment
    , withPinnedInConfig
    , withPipelineId
    , withPipelineName
    , withPublic
    , withReapTime
    , withResourceName
    , withShortJobId
    , withShortResourceId
    , withTeamName
    )

import Browser.Dom
import Concourse
import Concourse.BuildStatus as BuildStatus
import Dashboard.Group.Models
import Dict exposing (Dict)
import HoverState
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


resource : Maybe String -> Concourse.Resource
resource pinnedVersion =
    { teamName = teamName
    , pipelineName = pipelineName
    , pipelineId = pipelineId
    , name = resourceName
    , failingToCheck = False
    , checkError = ""
    , checkSetupError = ""
    , lastChecked = Nothing
    , pinnedVersion = Maybe.map version pinnedVersion
    , pinnedInConfig = False
    , pinComment = Nothing
    , icon = Nothing
    }


withLastChecked : Maybe Time.Posix -> { r | lastChecked : Maybe Time.Posix } -> { r | lastChecked : Maybe Time.Posix }
withLastChecked t r =
    { r | lastChecked = t }


withCheckError : String -> { r | checkError : String } -> { r | checkError : String }
withCheckError e r =
    { r | checkError = e }


withFailingToCheck : Bool -> { r | failingToCheck : Bool } -> { r | failingToCheck : Bool }
withFailingToCheck f r =
    { r | failingToCheck = f }


withPinnedInConfig : Bool -> { r | pinnedInConfig : Bool } -> { r | pinnedInConfig : Bool }
withPinnedInConfig p r =
    { r | pinnedInConfig = p }


withPinComment : Maybe String -> { r | pinComment : Maybe String } -> { r | pinComment : Maybe String }
withPinComment p r =
    { r | pinComment = p }


withIcon : Maybe String -> { r | icon : Maybe String } -> { r | icon : Maybe String }
withIcon i r =
    { r | icon = i }


pipeline : String -> Int -> Concourse.Pipeline
pipeline team id =
    { id = id
    , name = "pipeline-" ++ String.fromInt id
    , instanceVars = Dict.empty
    , paused = False
    , archived = False
    , public = True
    , teamName = team
    , groups = []
    , backgroundImage = Maybe.Nothing
    }


dashboardPipeline : Int -> Bool -> Dashboard.Group.Models.Pipeline
dashboardPipeline id public =
    { id = id
    , name = pipelineName
    , instanceVars = Dict.empty
    , teamName = teamName
    , public = public
    , isToggleLoading = False
    , isVisibilityLoading = False
    , paused = False
    , archived = False
    , stale = False
    , jobsDisabled = False
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


withBackgroundImage : String -> { r | backgroundImage : Maybe String } -> { r | backgroundImage : Maybe String }
withBackgroundImage bg p =
    { p | backgroundImage = Just bg }


withInstanceVars : Dict String Concourse.JsonValue -> { r | instanceVars : Dict String Concourse.JsonValue } -> { r | instanceVars : Dict String Concourse.JsonValue }
withInstanceVars instanceVars p =
    { p | instanceVars = instanceVars }


job : Int -> Concourse.Job
job pipelineID =
    { name = jobName
    , pipelineId = pipelineID
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


withDisableManualTrigger : Bool -> { r | disableManualTrigger : Bool } -> { r | disableManualTrigger : Bool }
withDisableManualTrigger disableManualTrigger p =
    { p | disableManualTrigger = disableManualTrigger }


withTeamName : String -> { r | teamName : String } -> { r | teamName : String }
withTeamName name p =
    { p | teamName = name }


withPipelineName : String -> { r | pipelineName : String } -> { r | pipelineName : String }
withPipelineName name p =
    { p | pipelineName = name }


withJobName : String -> { r | jobName : String } -> { r | jobName : String }
withJobName name p =
    { p | jobName = name }


withJob : Maybe Concourse.JobIdentifier -> { r | job : Maybe Concourse.JobIdentifier } -> { r | job : Maybe Concourse.JobIdentifier }
withJob j b =
    { b | job = j }


withResourceName : String -> { r | resourceName : String } -> { r | resourceName : String }
withResourceName name p =
    { p | resourceName = name }


withBuildName : String -> { r | buildName : String } -> { r | buildName : String }
withBuildName name p =
    { p | buildName = name }


withFinishedBuild : Maybe Concourse.Build -> { r | finishedBuild : Maybe Concourse.Build } -> { r | finishedBuild : Maybe Concourse.Build }
withFinishedBuild build j =
    { j | finishedBuild = build }


withNextBuild : Maybe Concourse.Build -> { r | nextBuild : Maybe Concourse.Build } -> { r | nextBuild : Maybe Concourse.Build }
withNextBuild build j =
    { j | nextBuild = build }


withId : Int -> { r | id : Int } -> { r | id : Int }
withId id j =
    { j | id = id }


type alias Duration =
    { startedAt : Maybe Time.Posix
    , finishedAt : Maybe Time.Posix
    }


withDuration : Duration -> { r | duration : Duration } -> { r | duration : Duration }
withDuration d b =
    { b | duration = d }


withReapTime : Maybe Time.Posix -> { r | reapTime : Maybe Time.Posix } -> { r | reapTime : Maybe Time.Posix }
withReapTime t b =
    { b | reapTime = t }


jobName =
    "job"


teamName =
    "team"


pipelineName =
    "pipeline"


resourceName =
    "resource"


buildName =
    "1"


withShortJobId =
    withJobName "j"


withShortResourceId =
    withResourceName "r"


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


pipelineId : Concourse.PipelineIdentifier
pipelineId =
    1


withPipelineId : Concourse.DatabaseID -> { r | pipelineId : Concourse.DatabaseID } -> { r | pipelineId : Concourse.DatabaseID }
withPipelineId id p =
    { p | pipelineId = id }


jobId : Concourse.JobIdentifier
jobId =
    { pipelineId = pipelineId
    , jobName = jobName
    }


shortJobId : Concourse.JobIdentifier
shortJobId =
    jobId |> withShortJobId


resourceId : Concourse.ResourceIdentifier
resourceId =
    { pipelineId = pipelineId
    , resourceName = resourceName
    }


shortResourceId : Concourse.ResourceIdentifier
shortResourceId =
    resourceId |> withShortResourceId


resourceVersionId : Int -> Concourse.VersionedResourceIdentifier
resourceVersionId v =
    { pipelineId = pipelineId
    , resourceName = resourceName
    , versionID = v
    }



-- jobBuildId is really shortJobBuildId, but since jobBuild returns a short jobId,
-- it would be weird for jobBuildId to not represent jobBuild


jobBuildId : Concourse.JobBuildIdentifier
jobBuildId =
    longJobBuildId |> withShortJobId


longJobBuildId : Concourse.JobBuildIdentifier
longJobBuildId =
    { pipelineId = pipelineId
    , jobName = jobName
    , buildName = buildName
    }


jobBuild : BuildStatus.BuildStatus -> Concourse.Build
jobBuild status =
    { id = 1
    , name = buildName
    , teamName = "t"
    , job = Just (jobId |> withShortJobId)
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


withHovered : HoverState.HoverState -> { r | hovered : HoverState.HoverState } -> { r | hovered : HoverState.HoverState }
withHovered hs v =
    { v | hovered = hs }
