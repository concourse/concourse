module Data exposing
    ( build
    , dashboardPipeline
    , elementPosition
    , httpForbidden
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
    , pipelineDatabaseId
    , pipelineId
    , pipelineName
    , resource
    , resourceId
    , resourceName
    , resourceVersionId
    , shortJobId
    , shortPipelineId
    , shortResourceId
    , teamName
    , version
    , versionedResource
    , withArchived
    , withBackgroundImage
    , withBuild
    , withBuildName
    , withCheckError
    , withDisableManualTrigger
    , withDuration
    , withFailingToCheck
    , withFinishedBuild
    , withGroups
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
    , withPipelineInstanceVars
    , withPipelineName
    , withPublic
    , withReapTime
    , withResourceName
    , withShortJobId
    , withShortPipelineId
    , withShortResourceId
    , withTeamName
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


httpForbidden : Result Http.Error a
httpForbidden =
    Err <|
        Http.BadStatus
            { url = "http://example.com"
            , status =
                { code = 403
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


resource : Maybe String -> Concourse.Resource
resource pinnedVersion =
    { teamName = teamName
    , pipelineId = pipelineDatabaseId
    , pipelineName = pipelineName
    , pipelineInstanceVars = Dict.empty
    , name = resourceName
    , lastChecked = Nothing
    , pinnedVersion = Maybe.map version pinnedVersion
    , pinnedInConfig = False
    , pinComment = Nothing
    , icon = Nothing
    , build = Nothing
    }


pipeline : Concourse.TeamName -> Int -> Concourse.Pipeline
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


dashboardPipeline : Concourse.TeamName -> Int -> Dashboard.Group.Models.Pipeline
dashboardPipeline team id =
    { id = id
    , name = "pipeline-" ++ String.fromInt id
    , instanceVars = Dict.empty
    , teamName = team
    , public = True
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


withPipelineInstanceVars : Dict String Concourse.JsonValue -> { r | pipelineInstanceVars : Dict String Concourse.JsonValue } -> { r | pipelineInstanceVars : Dict String Concourse.JsonValue }
withPipelineInstanceVars pipelineInstanceVars p =
    { p | pipelineInstanceVars = pipelineInstanceVars }


withJob : Maybe Concourse.JobIdentifier -> { r | job : Maybe Concourse.JobIdentifier } -> { r | job : Maybe Concourse.JobIdentifier }
withJob j b =
    { b | job = j }


job : Int -> Concourse.Job
job pipelineID =
    { name = jobName
    , pipelineId = pipelineID
    , pipelineName = "pipeline-" ++ String.fromInt pipelineID
    , pipelineInstanceVars = Dict.empty
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


withResourceName : String -> { r | resourceName : String } -> { r | resourceName : String }
withResourceName name p =
    { p | resourceName = name }


withBuildName : String -> { r | buildName : String } -> { r | buildName : String }
withBuildName name p =
    { p | buildName = name }


withFinishedBuild : Maybe Concourse.Build -> { r | finishedBuild : Maybe Concourse.Build } -> { r | finishedBuild : Maybe Concourse.Build }
withFinishedBuild b j =
    { j | finishedBuild = b }


withNextBuild : Maybe Concourse.Build -> { r | nextBuild : Maybe Concourse.Build } -> { r | nextBuild : Maybe Concourse.Build }
withNextBuild b j =
    { j | nextBuild = b }


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


withShortPipelineId =
    withPipelineName "p"
        >> withTeamName "t"


withShortJobId =
    withShortPipelineId >> withJobName "j"


withShortResourceId =
    withShortPipelineId >> withResourceName "r"


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
    { teamName = teamName
    , pipelineName = pipelineName
    , pipelineInstanceVars = Dict.empty
    }


pipelineDatabaseId : Concourse.DatabaseID
pipelineDatabaseId =
    1


withPipelineId : Concourse.DatabaseID -> { r | pipelineId : Concourse.DatabaseID } -> { r | pipelineId : Concourse.DatabaseID }
withPipelineId id p =
    { p | pipelineId = id }


shortPipelineId : Concourse.PipelineIdentifier
shortPipelineId =
    pipelineId |> withShortPipelineId


jobId : Concourse.JobIdentifier
jobId =
    { teamName = teamName
    , pipelineName = pipelineName
    , pipelineInstanceVars = Dict.empty
    , jobName = jobName
    }


shortJobId : Concourse.JobIdentifier
shortJobId =
    jobId |> withShortJobId


resourceId : Concourse.ResourceIdentifier
resourceId =
    { teamName = teamName
    , pipelineName = pipelineName
    , pipelineInstanceVars = Dict.empty
    , resourceName = resourceName
    }


shortResourceId : Concourse.ResourceIdentifier
shortResourceId =
    resourceId |> withShortResourceId


resourceVersionId : Int -> Concourse.VersionedResourceIdentifier
resourceVersionId v =
    { teamName = teamName
    , pipelineName = pipelineName
    , pipelineInstanceVars = Dict.empty
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
    { teamName = teamName
    , pipelineName = pipelineName
    , pipelineInstanceVars = Dict.empty
    , jobName = jobName
    , buildName = buildName
    }


build : BuildStatus.BuildStatus -> Concourse.Build
build status =
    { id = 1
    , name = buildName
    , teamName = teamName
    , job = Nothing
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


withBuild : a -> { r | build : a } -> { r | build : a }
withBuild b r =
    { r | build = b }


jobBuild : BuildStatus.BuildStatus -> Concourse.Build
jobBuild status =
    { id = 1
    , name = buildName
    , teamName = teamName
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
