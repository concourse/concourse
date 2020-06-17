module Concourse exposing
    ( AuthSession
    , AuthToken
    , Build
    , BuildDuration
    , BuildId
    , BuildName
    , BuildPlan
    , BuildPrep
    , BuildPrepStatus(..)
    , BuildResources
    , BuildResourcesInput
    , BuildResourcesOutput
    , BuildStep(..)
    , CSRFToken
    , Cause
    , Check
    , CheckIdentifier
    , CheckStatus(..)
    , ClusterInfo
    , HookedPlan
    , Job
    , JobBuildIdentifier
    , JobIdentifier
    , JobInput
    , JobName
    , JobOutput
    , Metadata
    , MetadataField
    , Pipeline
    , PipelineGroup
    , PipelineIdentifier
    , PipelineName
    , Resource
    , ResourceIdentifier
    , Team
    , TeamName
    , User
    , Version
    , VersionedResource
    , VersionedResourceIdentifier
    , csrfTokenHeaderName
    , customDecoder
    , decodeAuthToken
    , decodeBuild
    , decodeBuildPlan
    , decodeBuildPrep
    , decodeBuildResources
    , decodeCause
    , decodeCheck
    , decodeInfo
    , decodeJob
    , decodeMetadata
    , decodePipeline
    , decodePipelineIdentifier
    , decodeResource
    , decodeTeam
    , decodeUser
    , decodeVersion
    , decodeVersionedResource
    , emptyBuildResources
    , encodeBuild
    , encodeJob
    , encodePipeline
    , encodePipelineIdentifier
    , encodeTeam
    , retrieveCSRFToken
    )

import Array exposing (Array)
import Concourse.BuildStatus exposing (BuildStatus)
import Dict exposing (Dict)
import Json.Decode
import Json.Decode.Extra exposing (andMap)
import Json.Encode
import Time



-- AuthToken


type alias AuthToken =
    String


decodeAuthToken : Json.Decode.Decoder AuthToken
decodeAuthToken =
    customDecoder
        (Json.Decode.succeed (\a b -> ( a, b ))
            |> andMap (Json.Decode.field "type" Json.Decode.string)
            |> andMap (Json.Decode.field "value" Json.Decode.string)
        )
        authTokenFromTuple


authTokenFromTuple : ( String, String ) -> Result Json.Decode.Error AuthToken
authTokenFromTuple ( t, token ) =
    case t of
        "Bearer" ->
            Ok token

        _ ->
            Err <| Json.Decode.Failure "unknown token type" <| Json.Encode.string token



-- CSRF token


type alias CSRFToken =
    String


csrfTokenHeaderName : String
csrfTokenHeaderName =
    "X-Csrf-Token"


retrieveCSRFToken : Dict String String -> Result String CSRFToken
retrieveCSRFToken headers =
    Dict.get (String.toLower csrfTokenHeaderName) (keysToLower headers) |> Result.fromMaybe "error CSRFToken not found"


keysToLower : Dict String a -> Dict String a
keysToLower =
    Dict.fromList << List.map fstToLower << Dict.toList


fstToLower : ( String, a ) -> ( String, a )
fstToLower ( x, y ) =
    ( String.toLower x, y )


type alias AuthSession =
    { authToken : AuthToken
    , csrfToken : CSRFToken
    }



-- Build


type alias BuildId =
    Int


type alias BuildName =
    String


type alias JobBuildIdentifier =
    { teamName : TeamName
    , pipelineName : PipelineName
    , jobName : JobName
    , buildName : BuildName
    }


type alias Build =
    { id : BuildId
    , name : BuildName
    , job : Maybe JobIdentifier
    , status : BuildStatus
    , duration : BuildDuration
    , reapTime : Maybe Time.Posix
    }


type alias BuildDuration =
    { startedAt : Maybe Time.Posix
    , finishedAt : Maybe Time.Posix
    }


encodeBuild : Build -> Json.Encode.Value
encodeBuild build =
    Json.Encode.object
        ([ ( "id", build.id |> Json.Encode.int ) |> Just
         , ( "name", build.name |> Json.Encode.string ) |> Just
         , optionalField "team_name" Json.Encode.string (build.job |> Maybe.map .teamName)
         , optionalField "pipeline_name" Json.Encode.string (build.job |> Maybe.map .pipelineName)
         , optionalField "job_name" Json.Encode.string (build.job |> Maybe.map .jobName)
         , ( "status", build.status |> Concourse.BuildStatus.encodeBuildStatus ) |> Just
         , optionalField "start_time" (secondsFromDate >> Json.Encode.int) build.duration.startedAt
         , optionalField "end_time" (secondsFromDate >> Json.Encode.int) build.duration.finishedAt
         , optionalField "reap_time" (secondsFromDate >> Json.Encode.int) build.reapTime
         ]
            |> List.filterMap identity
        )


encodeMaybeBuild : Maybe Build -> Json.Encode.Value
encodeMaybeBuild maybeBuild =
    case maybeBuild of
        Nothing ->
            Json.Encode.null

        Just build ->
            encodeBuild build


decodeBuild : Json.Decode.Decoder Build
decodeBuild =
    Json.Decode.succeed Build
        |> andMap (Json.Decode.field "id" Json.Decode.int)
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap
            (Json.Decode.maybe
                (Json.Decode.succeed JobIdentifier
                    |> andMap (Json.Decode.field "team_name" Json.Decode.string)
                    |> andMap (Json.Decode.field "pipeline_name" Json.Decode.string)
                    |> andMap (Json.Decode.field "job_name" Json.Decode.string)
                )
            )
        |> andMap (Json.Decode.field "status" Concourse.BuildStatus.decodeBuildStatus)
        |> andMap
            (Json.Decode.succeed BuildDuration
                |> andMap (Json.Decode.maybe (Json.Decode.field "start_time" (Json.Decode.map dateFromSeconds Json.Decode.int)))
                |> andMap (Json.Decode.maybe (Json.Decode.field "end_time" (Json.Decode.map dateFromSeconds Json.Decode.int)))
            )
        |> andMap (Json.Decode.maybe (Json.Decode.field "reap_time" (Json.Decode.map dateFromSeconds Json.Decode.int)))



-- BuildPrep


type alias BuildPrep =
    { pausedPipeline : BuildPrepStatus
    , pausedJob : BuildPrepStatus
    , maxRunningBuilds : BuildPrepStatus
    , inputs : Dict String BuildPrepStatus
    , inputsSatisfied : BuildPrepStatus
    , missingInputReasons : Dict String String
    }


type BuildPrepStatus
    = BuildPrepStatusUnknown
    | BuildPrepStatusBlocking
    | BuildPrepStatusNotBlocking


decodeBuildPrep : Json.Decode.Decoder BuildPrep
decodeBuildPrep =
    Json.Decode.succeed BuildPrep
        |> andMap (Json.Decode.field "paused_pipeline" decodeBuildPrepStatus)
        |> andMap (Json.Decode.field "paused_job" decodeBuildPrepStatus)
        |> andMap (Json.Decode.field "max_running_builds" decodeBuildPrepStatus)
        |> andMap (Json.Decode.field "inputs" <| Json.Decode.dict decodeBuildPrepStatus)
        |> andMap (Json.Decode.field "inputs_satisfied" decodeBuildPrepStatus)
        |> andMap (defaultTo Dict.empty <| Json.Decode.field "missing_input_reasons" <| Json.Decode.dict Json.Decode.string)


decodeBuildPrepStatus : Json.Decode.Decoder BuildPrepStatus
decodeBuildPrepStatus =
    customDecoder Json.Decode.string <|
        \status ->
            case status of
                "unknown" ->
                    Ok BuildPrepStatusUnknown

                "blocking" ->
                    Ok BuildPrepStatusBlocking

                "not_blocking" ->
                    Ok BuildPrepStatusNotBlocking

                unknown ->
                    Err <| Json.Decode.Failure "unknown build preparation status" <| Json.Encode.string unknown



-- BuildResources


type alias BuildResources =
    { inputs : List BuildResourcesInput
    , outputs : List BuildResourcesOutput
    }


type alias BuildResourcesInput =
    { name : String
    , version : Version
    , firstOccurrence : Bool
    }


type alias BuildResourcesOutput =
    { name : String
    , version : Version
    }


emptyBuildResources : BuildResources
emptyBuildResources =
    { inputs = []
    , outputs = []
    }


decodeBuildResources : Json.Decode.Decoder BuildResources
decodeBuildResources =
    Json.Decode.succeed BuildResources
        |> andMap (Json.Decode.field "inputs" <| Json.Decode.list decodeResourcesInput)
        |> andMap (Json.Decode.field "outputs" <| Json.Decode.list decodeResourcesOutput)


decodeResourcesInput : Json.Decode.Decoder BuildResourcesInput
decodeResourcesInput =
    Json.Decode.succeed BuildResourcesInput
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "version" decodeVersion)
        |> andMap (Json.Decode.field "first_occurrence" Json.Decode.bool)


decodeResourcesOutput : Json.Decode.Decoder BuildResourcesOutput
decodeResourcesOutput =
    Json.Decode.succeed BuildResourcesOutput
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "version" <| Json.Decode.dict Json.Decode.string)



-- BuildPlan


type alias BuildPlan =
    { id : String
    , step : BuildStep
    }


type alias StepName =
    String


type BuildStep
    = BuildStepTask StepName
    | BuildStepSetPipeline StepName
    | BuildStepLoadVar StepName
    | BuildStepArtifactInput StepName
    | BuildStepGet StepName (Maybe Version)
    | BuildStepArtifactOutput StepName
    | BuildStepPut StepName
    | BuildStepAggregate (Array BuildPlan)
    | BuildStepInParallel (Array BuildPlan)
    | BuildStepDo (Array BuildPlan)
    | BuildStepOnSuccess HookedPlan
    | BuildStepOnFailure HookedPlan
    | BuildStepOnAbort HookedPlan
    | BuildStepOnError HookedPlan
    | BuildStepEnsure HookedPlan
    | BuildStepTry BuildPlan
    | BuildStepRetry (Array BuildPlan)
    | BuildStepTimeout BuildPlan


type alias HookedPlan =
    { step : BuildPlan
    , hook : BuildPlan
    }


decodeBuildPlan : Json.Decode.Decoder BuildPlan
decodeBuildPlan =
    Json.Decode.at [ "plan" ] <|
        decodeBuildPlan_


decodeBuildPlan_ : Json.Decode.Decoder BuildPlan
decodeBuildPlan_ =
    Json.Decode.succeed BuildPlan
        |> andMap (Json.Decode.field "id" Json.Decode.string)
        |> andMap
            (Json.Decode.oneOf
                -- buckle up
                [ Json.Decode.field "task" <|
                    lazy (\_ -> decodeBuildStepTask)
                , Json.Decode.field "get" <|
                    lazy (\_ -> decodeBuildStepGet)
                , Json.Decode.field "artifact_input" <|
                    lazy (\_ -> decodeBuildStepArtifactInput)
                , Json.Decode.field "put" <|
                    lazy (\_ -> decodeBuildStepPut)
                , Json.Decode.field "artifact_output" <|
                    lazy (\_ -> decodeBuildStepArtifactOutput)
                , Json.Decode.field "dependent_get" <|
                    lazy (\_ -> decodeBuildStepGet)
                , Json.Decode.field "aggregate" <|
                    lazy (\_ -> decodeBuildStepAggregate)
                , Json.Decode.field "in_parallel" <|
                    lazy (\_ -> decodeBuildStepInParallel)
                , Json.Decode.field "do" <|
                    lazy (\_ -> decodeBuildStepDo)
                , Json.Decode.field "on_success" <|
                    lazy (\_ -> decodeBuildStepOnSuccess)
                , Json.Decode.field "on_failure" <|
                    lazy (\_ -> decodeBuildStepOnFailure)
                , Json.Decode.field "on_abort" <|
                    lazy (\_ -> decodeBuildStepOnAbort)
                , Json.Decode.field "on_error" <|
                    lazy (\_ -> decodeBuildStepOnError)
                , Json.Decode.field "ensure" <|
                    lazy (\_ -> decodeBuildStepEnsure)
                , Json.Decode.field "try" <|
                    lazy (\_ -> decodeBuildStepTry)
                , Json.Decode.field "retry" <|
                    lazy (\_ -> decodeBuildStepRetry)
                , Json.Decode.field "timeout" <|
                    lazy (\_ -> decodeBuildStepTimeout)
                , Json.Decode.field "set_pipeline" <|
                    lazy (\_ -> decodeBuildSetPipeline)
                , Json.Decode.field "load_var" <|
                    lazy (\_ -> decodeBuildStepLoadVar)
                ]
            )


decodeBuildStepTask : Json.Decode.Decoder BuildStep
decodeBuildStepTask =
    Json.Decode.succeed BuildStepTask
        |> andMap (Json.Decode.field "name" Json.Decode.string)


decodeBuildStepArtifactInput : Json.Decode.Decoder BuildStep
decodeBuildStepArtifactInput =
    Json.Decode.succeed BuildStepArtifactInput
        |> andMap (Json.Decode.field "name" Json.Decode.string)


decodeBuildStepGet : Json.Decode.Decoder BuildStep
decodeBuildStepGet =
    Json.Decode.succeed BuildStepGet
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.maybe <| Json.Decode.field "version" decodeVersion)


decodeBuildStepArtifactOutput : Json.Decode.Decoder BuildStep
decodeBuildStepArtifactOutput =
    Json.Decode.succeed BuildStepArtifactOutput
        |> andMap (Json.Decode.field "name" Json.Decode.string)


decodeBuildStepPut : Json.Decode.Decoder BuildStep
decodeBuildStepPut =
    Json.Decode.succeed BuildStepPut
        |> andMap (Json.Decode.field "name" Json.Decode.string)


decodeBuildStepAggregate : Json.Decode.Decoder BuildStep
decodeBuildStepAggregate =
    Json.Decode.succeed BuildStepAggregate
        |> andMap (Json.Decode.array (lazy (\_ -> decodeBuildPlan_)))


decodeBuildStepInParallel : Json.Decode.Decoder BuildStep
decodeBuildStepInParallel =
    Json.Decode.succeed BuildStepInParallel
        |> andMap (Json.Decode.field "steps" <| Json.Decode.array (lazy (\_ -> decodeBuildPlan_)))


decodeBuildStepDo : Json.Decode.Decoder BuildStep
decodeBuildStepDo =
    Json.Decode.succeed BuildStepDo
        |> andMap (Json.Decode.array (lazy (\_ -> decodeBuildPlan_)))


decodeBuildStepOnSuccess : Json.Decode.Decoder BuildStep
decodeBuildStepOnSuccess =
    Json.Decode.map BuildStepOnSuccess
        (Json.Decode.succeed HookedPlan
            |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan_))
            |> andMap (Json.Decode.field "on_success" <| lazy (\_ -> decodeBuildPlan_))
        )


decodeBuildStepOnFailure : Json.Decode.Decoder BuildStep
decodeBuildStepOnFailure =
    Json.Decode.map BuildStepOnFailure
        (Json.Decode.succeed HookedPlan
            |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan_))
            |> andMap (Json.Decode.field "on_failure" <| lazy (\_ -> decodeBuildPlan_))
        )


decodeBuildStepOnAbort : Json.Decode.Decoder BuildStep
decodeBuildStepOnAbort =
    Json.Decode.map BuildStepOnAbort
        (Json.Decode.succeed HookedPlan
            |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan_))
            |> andMap (Json.Decode.field "on_abort" <| lazy (\_ -> decodeBuildPlan_))
        )


decodeBuildStepOnError : Json.Decode.Decoder BuildStep
decodeBuildStepOnError =
    Json.Decode.map BuildStepOnError
        (Json.Decode.succeed HookedPlan
            |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan_))
            |> andMap (Json.Decode.field "on_error" <| lazy (\_ -> decodeBuildPlan_))
        )


decodeBuildStepEnsure : Json.Decode.Decoder BuildStep
decodeBuildStepEnsure =
    Json.Decode.map BuildStepEnsure
        (Json.Decode.succeed HookedPlan
            |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan_))
            |> andMap (Json.Decode.field "ensure" <| lazy (\_ -> decodeBuildPlan_))
        )


decodeBuildStepTry : Json.Decode.Decoder BuildStep
decodeBuildStepTry =
    Json.Decode.succeed BuildStepTry
        |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan_))


decodeBuildStepRetry : Json.Decode.Decoder BuildStep
decodeBuildStepRetry =
    Json.Decode.succeed BuildStepRetry
        |> andMap (Json.Decode.array (lazy (\_ -> decodeBuildPlan_)))


decodeBuildStepTimeout : Json.Decode.Decoder BuildStep
decodeBuildStepTimeout =
    Json.Decode.succeed BuildStepTimeout
        |> andMap (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan_))


decodeBuildSetPipeline : Json.Decode.Decoder BuildStep
decodeBuildSetPipeline =
    Json.Decode.succeed BuildStepSetPipeline
        |> andMap (Json.Decode.field "name" Json.Decode.string)


decodeBuildStepLoadVar : Json.Decode.Decoder BuildStep
decodeBuildStepLoadVar =
    Json.Decode.succeed BuildStepLoadVar
        |> andMap (Json.Decode.field "name" Json.Decode.string)



-- Info


type alias ClusterInfo =
    { version : String
    , clusterName : String
    }


decodeInfo : Json.Decode.Decoder ClusterInfo
decodeInfo =
    Json.Decode.succeed ClusterInfo
        |> andMap (Json.Decode.field "version" Json.Decode.string)
        |> andMap (defaultTo "" <| Json.Decode.field "cluster_name" Json.Decode.string)



-- Job


type alias JobName =
    String


type alias JobIdentifier =
    { teamName : TeamName
    , pipelineName : PipelineName
    , jobName : JobName
    }


type alias Job =
    { name : JobName
    , pipelineName : PipelineName
    , teamName : TeamName
    , nextBuild : Maybe Build
    , finishedBuild : Maybe Build
    , transitionBuild : Maybe Build
    , paused : Bool
    , disableManualTrigger : Bool
    , inputs : List JobInput
    , outputs : List JobOutput
    , groups : List String
    }


type alias JobInput =
    { name : String
    , resource : String
    , passed : List String
    , trigger : Bool
    }


type alias JobOutput =
    { name : String
    , resource : String
    }


encodeJob : Job -> Json.Encode.Value
encodeJob job =
    Json.Encode.object
        [ ( "name", job.name |> Json.Encode.string )
        , ( "pipeline_name", job.pipelineName |> Json.Encode.string )
        , ( "team_name", job.teamName |> Json.Encode.string )
        , ( "next_build", job.nextBuild |> encodeMaybeBuild )
        , ( "finished_build", job.finishedBuild |> encodeMaybeBuild )
        , ( "transition_build", job.finishedBuild |> encodeMaybeBuild )
        , ( "paused", job.paused |> Json.Encode.bool )
        , ( "disable_manual_trigger", job.paused |> Json.Encode.bool )
        , ( "disable_manual_trigger", job.disableManualTrigger |> Json.Encode.bool )
        , ( "inputs", job.inputs |> Json.Encode.list encodeJobInput )
        , ( "outputs", job.outputs |> Json.Encode.list encodeJobOutput )
        , ( "groups", job.groups |> Json.Encode.list Json.Encode.string )
        ]


decodeJob : Json.Decode.Decoder Job
decodeJob =
    Json.Decode.succeed Job
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "pipeline_name" Json.Decode.string)
        |> andMap (Json.Decode.field "team_name" Json.Decode.string)
        |> andMap (Json.Decode.maybe (Json.Decode.field "next_build" decodeBuild))
        |> andMap (Json.Decode.maybe (Json.Decode.field "finished_build" decodeBuild))
        |> andMap (Json.Decode.maybe (Json.Decode.field "transition_build" decodeBuild))
        |> andMap (defaultTo False <| Json.Decode.field "paused" Json.Decode.bool)
        |> andMap (defaultTo False <| Json.Decode.field "disable_manual_trigger" Json.Decode.bool)
        |> andMap (defaultTo [] <| Json.Decode.field "inputs" <| Json.Decode.list decodeJobInput)
        |> andMap (defaultTo [] <| Json.Decode.field "outputs" <| Json.Decode.list decodeJobOutput)
        |> andMap (defaultTo [] <| Json.Decode.field "groups" <| Json.Decode.list Json.Decode.string)


encodeJobInput : JobInput -> Json.Encode.Value
encodeJobInput jobInput =
    Json.Encode.object
        [ ( "name", jobInput.name |> Json.Encode.string )
        , ( "resource", jobInput.resource |> Json.Encode.string )
        , ( "passed", jobInput.passed |> Json.Encode.list Json.Encode.string )
        , ( "trigger", jobInput.trigger |> Json.Encode.bool )
        ]


decodeJobInput : Json.Decode.Decoder JobInput
decodeJobInput =
    Json.Decode.succeed JobInput
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "resource" Json.Decode.string)
        |> andMap (defaultTo [] <| Json.Decode.field "passed" <| Json.Decode.list Json.Decode.string)
        |> andMap (defaultTo False <| Json.Decode.field "trigger" Json.Decode.bool)


encodeJobOutput : JobOutput -> Json.Encode.Value
encodeJobOutput jobOutput =
    Json.Encode.object
        [ ( "name", jobOutput.name |> Json.Encode.string )
        , ( "resource", jobOutput.resource |> Json.Encode.string )
        ]


decodeJobOutput : Json.Decode.Decoder JobOutput
decodeJobOutput =
    Json.Decode.succeed JobOutput
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "resource" Json.Decode.string)



-- Pipeline


type alias PipelineName =
    String


type alias PipelineIdentifier =
    { teamName : TeamName
    , pipelineName : PipelineName
    }


type alias Pipeline =
    { id : Int
    , name : PipelineName
    , paused : Bool
    , archived : Bool
    , public : Bool
    , teamName : TeamName
    , groups : List PipelineGroup
    }


type alias PipelineGroup =
    { name : String
    , jobs : List String
    , resources : List String
    }


encodePipelineIdentifier : PipelineIdentifier -> Json.Encode.Value
encodePipelineIdentifier pipelineIdentifier =
    Json.Encode.object
        [ ( "team_name", pipelineIdentifier.teamName |> Json.Encode.string )
        , ( "name", pipelineIdentifier.pipelineName |> Json.Encode.string )
        ]


decodePipelineIdentifier : Json.Decode.Decoder PipelineIdentifier
decodePipelineIdentifier =
    Json.Decode.succeed PipelineIdentifier
        |> andMap (Json.Decode.field "team_name" Json.Decode.string)
        |> andMap (Json.Decode.field "name" Json.Decode.string)


encodePipeline : Pipeline -> Json.Encode.Value
encodePipeline pipeline =
    Json.Encode.object
        [ ( "id", pipeline.id |> Json.Encode.int )
        , ( "name", pipeline.name |> Json.Encode.string )
        , ( "paused", pipeline.paused |> Json.Encode.bool )
        , ( "archived", pipeline.archived |> Json.Encode.bool )
        , ( "public", pipeline.public |> Json.Encode.bool )
        , ( "team_name", pipeline.teamName |> Json.Encode.string )
        , ( "groups", pipeline.groups |> Json.Encode.list encodePipelineGroup )
        ]


decodePipeline : Json.Decode.Decoder Pipeline
decodePipeline =
    Json.Decode.succeed Pipeline
        |> andMap (Json.Decode.field "id" Json.Decode.int)
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "paused" Json.Decode.bool)
        |> andMap (Json.Decode.field "archived" Json.Decode.bool)
        |> andMap (Json.Decode.field "public" Json.Decode.bool)
        |> andMap (Json.Decode.field "team_name" Json.Decode.string)
        |> andMap (defaultTo [] <| Json.Decode.field "groups" (Json.Decode.list decodePipelineGroup))


encodePipelineGroup : PipelineGroup -> Json.Encode.Value
encodePipelineGroup pipelineGroup =
    Json.Encode.object
        [ ( "name", pipelineGroup.name |> Json.Encode.string )
        , ( "jobs", pipelineGroup.jobs |> Json.Encode.list Json.Encode.string )
        , ( "resources", pipelineGroup.resources |> Json.Encode.list Json.Encode.string )
        ]


decodePipelineGroup : Json.Decode.Decoder PipelineGroup
decodePipelineGroup =
    Json.Decode.succeed PipelineGroup
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (defaultTo [] <| Json.Decode.field "jobs" <| Json.Decode.list Json.Decode.string)
        |> andMap (defaultTo [] <| Json.Decode.field "resources" <| Json.Decode.list Json.Decode.string)



-- Resource


type alias Resource =
    { teamName : String
    , pipelineName : String
    , name : String
    , icon : Maybe String
    , failingToCheck : Bool
    , checkError : String
    , checkSetupError : String
    , lastChecked : Maybe Time.Posix
    , pinnedVersion : Maybe Version
    , pinnedInConfig : Bool
    , pinComment : Maybe String
    }


type alias ResourceIdentifier =
    { teamName : String
    , pipelineName : String
    , resourceName : String
    }


type alias CheckIdentifier =
    { teamName : String
    , pipelineName : String
    , resourceName : String
    , checkID : Int
    }


type alias VersionedResource =
    { id : Int
    , version : Version
    , metadata : Metadata
    , enabled : Bool
    }


type alias VersionedResourceIdentifier =
    { teamName : String
    , pipelineName : String
    , resourceName : String
    , versionID : Int
    }


type alias Check =
    { id : Int
    , status : CheckStatus
    , createTime : Maybe Time.Posix
    , startTime : Maybe Time.Posix
    , endTime : Maybe Time.Posix
    , checkError : Maybe String
    }


type CheckStatus
    = Started
    | Succeeded
    | Errored


decodeResource : Json.Decode.Decoder Resource
decodeResource =
    Json.Decode.succeed Resource
        |> andMap (Json.Decode.field "team_name" Json.Decode.string)
        |> andMap (Json.Decode.field "pipeline_name" Json.Decode.string)
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.maybe (Json.Decode.field "icon" Json.Decode.string))
        |> andMap (defaultTo False <| Json.Decode.field "failing_to_check" Json.Decode.bool)
        |> andMap (defaultTo "" <| Json.Decode.field "check_error" Json.Decode.string)
        |> andMap (defaultTo "" <| Json.Decode.field "check_setup_error" Json.Decode.string)
        |> andMap (Json.Decode.maybe (Json.Decode.field "last_checked" (Json.Decode.map dateFromSeconds Json.Decode.int)))
        |> andMap (Json.Decode.maybe (Json.Decode.field "pinned_version" decodeVersion))
        |> andMap (defaultTo False <| Json.Decode.field "pinned_in_config" Json.Decode.bool)
        |> andMap (Json.Decode.maybe (Json.Decode.field "pin_comment" Json.Decode.string))


decodeVersionedResource : Json.Decode.Decoder VersionedResource
decodeVersionedResource =
    Json.Decode.succeed VersionedResource
        |> andMap (Json.Decode.field "id" Json.Decode.int)
        |> andMap (Json.Decode.field "version" decodeVersion)
        |> andMap (defaultTo [] (Json.Decode.field "metadata" decodeMetadata))
        |> andMap (Json.Decode.field "enabled" Json.Decode.bool)


decodeCheck : Json.Decode.Decoder Check
decodeCheck =
    Json.Decode.succeed Check
        |> andMap (Json.Decode.field "id" Json.Decode.int)
        |> andMap (Json.Decode.field "status" decodeCheckStatus)
        |> andMap (Json.Decode.maybe (Json.Decode.field "create_time" (Json.Decode.map dateFromSeconds Json.Decode.int)))
        |> andMap (Json.Decode.maybe (Json.Decode.field "start_time" (Json.Decode.map dateFromSeconds Json.Decode.int)))
        |> andMap (Json.Decode.maybe (Json.Decode.field "end_time" (Json.Decode.map dateFromSeconds Json.Decode.int)))
        |> andMap (Json.Decode.maybe (Json.Decode.field "check_error" Json.Decode.string))


decodeCheckStatus : Json.Decode.Decoder CheckStatus
decodeCheckStatus =
    Json.Decode.string
        |> Json.Decode.andThen
            (\status ->
                case status of
                    "started" ->
                        Json.Decode.succeed Started

                    "succeeded" ->
                        Json.Decode.succeed Succeeded

                    "errored" ->
                        Json.Decode.succeed Errored

                    unknown ->
                        Json.Decode.fail <| "unknown check status: " ++ unknown
            )



-- Version


type alias Version =
    Dict String String


decodeVersion : Json.Decode.Decoder Version
decodeVersion =
    Json.Decode.dict Json.Decode.string



-- Metadata


type alias Metadata =
    List MetadataField


type alias MetadataField =
    { name : String
    , value : String
    }


decodeMetadata : Json.Decode.Decoder (List MetadataField)
decodeMetadata =
    Json.Decode.list decodeMetadataField


decodeMetadataField : Json.Decode.Decoder MetadataField
decodeMetadataField =
    Json.Decode.succeed MetadataField
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "value" Json.Decode.string)



-- Team


type alias TeamName =
    String


type alias Team =
    { id : Int
    , name : TeamName
    }


encodeTeam : Team -> Json.Encode.Value
encodeTeam team =
    Json.Encode.object
        [ ( "id", team.id |> Json.Encode.int )
        , ( "name", team.name |> Json.Encode.string )
        ]


decodeTeam : Json.Decode.Decoder Team
decodeTeam =
    Json.Decode.succeed Team
        |> andMap (Json.Decode.field "id" Json.Decode.int)
        |> andMap (Json.Decode.field "name" Json.Decode.string)



-- User


type alias User =
    { id : String
    , userName : String
    , name : String
    , email : String
    , isAdmin : Bool
    , teams : Dict String (List String)
    }


decodeUser : Json.Decode.Decoder User
decodeUser =
    Json.Decode.succeed User
        |> andMap (Json.Decode.field "user_id" Json.Decode.string)
        |> andMap (Json.Decode.field "user_name" Json.Decode.string)
        |> andMap (Json.Decode.field "name" Json.Decode.string)
        |> andMap (Json.Decode.field "email" Json.Decode.string)
        |> andMap (Json.Decode.field "is_admin" Json.Decode.bool)
        |> andMap (Json.Decode.field "teams" (Json.Decode.dict (Json.Decode.list Json.Decode.string)))



-- Cause


type alias Cause =
    { versionedResourceID : Int
    , buildID : Int
    }


decodeCause : Json.Decode.Decoder Cause
decodeCause =
    Json.Decode.succeed Cause
        |> andMap (Json.Decode.field "versioned_resource_id" Json.Decode.int)
        |> andMap (Json.Decode.field "build_id" Json.Decode.int)



-- Helpers


dateFromSeconds : Int -> Time.Posix
dateFromSeconds =
    Time.millisToPosix << (*) 1000


secondsFromDate : Time.Posix -> Int
secondsFromDate =
    Time.posixToMillis >> (\m -> m // 1000)


lazy : (() -> Json.Decode.Decoder a) -> Json.Decode.Decoder a
lazy thunk =
    customDecoder Json.Decode.value
        (\js -> Json.Decode.decodeValue (thunk ()) js)


defaultTo : a -> Json.Decode.Decoder a -> Json.Decode.Decoder a
defaultTo default =
    Json.Decode.map (Maybe.withDefault default) << Json.Decode.maybe


customDecoder : Json.Decode.Decoder b -> (b -> Result Json.Decode.Error a) -> Json.Decode.Decoder a
customDecoder decoder toResult =
    Json.Decode.andThen
        (\a ->
            case toResult a of
                Ok b ->
                    Json.Decode.succeed b

                Err err ->
                    Json.Decode.fail <| Json.Decode.errorToString err
        )
        decoder


optionalField : String -> (a -> Json.Encode.Value) -> Maybe a -> Maybe ( String, Json.Encode.Value )
optionalField field encoder =
    Maybe.map (\val -> ( field, encoder val ))
