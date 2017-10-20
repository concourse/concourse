module Concourse
    exposing
        ( AuthMethod(..)
        , decodeAuthMethod
        , AuthToken
        , decodeAuthToken
        , CSRFToken
        , retrieveCSRFToken
        , csrfTokenHeaderName
        , AuthSession
        , Build
        , BuildId
        , JobBuildIdentifier
        , BuildName
        , BuildDuration
        , decodeBuild
        , BuildPrep
        , BuildPrepStatus(..)
        , decodeBuildPrep
        , BuildResources
        , BuildResourcesInput
        , BuildResourcesOutput
        , decodeBuildResources
        , BuildStatus(..)
        , decodeBuildStatus
        , BuildPlan
        , BuildStep(..)
        , HookedPlan
        , decodeBuildPlan
        , Info
        , decodeInfo
        , Job
        , JobName
        , JobIdentifier
        , JobInput
        , JobOutput
        , decodeJob
        , Pipeline
        , PipelineName
        , PipelineIdentifier
        , PipelineGroup
        , decodePipeline
        , Metadata
        , MetadataField
        , decodeMetadata
        , Resource
        , ResourceIdentifier
        , VersionedResource
        , VersionedResourceIdentifier
        , decodeResource
        , decodeVersionedResource
        , Team
        , TeamName
        , decodeTeam
        , User
        , decodeUser
        , Version
        , decodeVersion
        )

import Array exposing (Array)
import Date exposing (Date)
import Dict exposing (Dict)
import Json.Decode
import Json.Decode.Extra exposing ((|:))


-- AuthMethod


type AuthMethod
    = AuthMethodBasic
    | AuthMethodOAuth OAuthAuthMethod


type alias OAuthAuthMethod =
    { displayName : String
    , authUrl : String
    }


decodeAuthMethod : Json.Decode.Decoder AuthMethod
decodeAuthMethod =
    customDecoder
        (Json.Decode.succeed (,,)
            |: (Json.Decode.field "type" Json.Decode.string)
            |: (Json.Decode.maybe <| Json.Decode.field "display_name" Json.Decode.string)
            |: (Json.Decode.maybe <| Json.Decode.field "auth_url" Json.Decode.string)
        )
        authMethodFromTuple


authMethodFromTuple : ( String, Maybe String, Maybe String ) -> Result String AuthMethod
authMethodFromTuple tuple =
    case tuple of
        ( "basic", _, _ ) ->
            Ok AuthMethodBasic

        ( "oauth", Just displayName, Just authUrl ) ->
            Ok (AuthMethodOAuth { displayName = displayName, authUrl = authUrl })

        ( "oauth", _, _ ) ->
            Err "missing fields in oauth auth method"

        _ ->
            Err "unknown value for auth method type"



-- AuthToken


type alias AuthToken =
    String


decodeAuthToken : Json.Decode.Decoder AuthToken
decodeAuthToken =
    customDecoder
        (Json.Decode.succeed (,)
            |: (Json.Decode.field "type" Json.Decode.string)
            |: (Json.Decode.field "value" Json.Decode.string)
        )
        authTokenFromTuple


authTokenFromTuple : ( String, String ) -> Result String AuthToken
authTokenFromTuple ( t, token ) =
    case t of
        "Bearer" ->
            Ok token

        _ ->
            Err "unknown token type"



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
    , url : String
    , name : BuildName
    , job : Maybe JobIdentifier
    , status : BuildStatus
    , duration : BuildDuration
    , reapTime : Maybe Date
    }


type BuildStatus
    = BuildStatusPending
    | BuildStatusStarted
    | BuildStatusSucceeded
    | BuildStatusFailed
    | BuildStatusErrored
    | BuildStatusAborted


type alias BuildDuration =
    { startedAt : Maybe Date
    , finishedAt : Maybe Date
    }


decodeBuild : Json.Decode.Decoder Build
decodeBuild =
    Json.Decode.succeed Build
        |: (Json.Decode.field "id" Json.Decode.int)
        |: (Json.Decode.field "url" Json.Decode.string)
        |: (Json.Decode.field "name" Json.Decode.string)
        |: (Json.Decode.maybe
                (Json.Decode.succeed JobIdentifier
                    |: (Json.Decode.field "team_name" Json.Decode.string)
                    |: (Json.Decode.field "pipeline_name" Json.Decode.string)
                    |: (Json.Decode.field "job_name" Json.Decode.string)
                )
           )
        |: (Json.Decode.field "status" decodeBuildStatus)
        |: (Json.Decode.succeed BuildDuration
                |: (Json.Decode.maybe (Json.Decode.field "start_time" (Json.Decode.map dateFromSeconds Json.Decode.float)))
                |: (Json.Decode.maybe (Json.Decode.field "end_time" (Json.Decode.map dateFromSeconds Json.Decode.float)))
           )
        |: (Json.Decode.maybe (Json.Decode.field "reap_time" (Json.Decode.map dateFromSeconds Json.Decode.float)))


decodeBuildStatus : Json.Decode.Decoder BuildStatus
decodeBuildStatus =
    customDecoder Json.Decode.string <|
        \status ->
            case status of
                "pending" ->
                    Ok BuildStatusPending

                "started" ->
                    Ok BuildStatusStarted

                "succeeded" ->
                    Ok BuildStatusSucceeded

                "failed" ->
                    Ok BuildStatusFailed

                "errored" ->
                    Ok BuildStatusErrored

                "aborted" ->
                    Ok BuildStatusAborted

                unknown ->
                    Err ("unknown build status: " ++ unknown)



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
        |: (Json.Decode.field "paused_pipeline" decodeBuildPrepStatus)
        |: (Json.Decode.field "paused_job" decodeBuildPrepStatus)
        |: (Json.Decode.field "max_running_builds" decodeBuildPrepStatus)
        |: (Json.Decode.field "inputs" <| Json.Decode.dict decodeBuildPrepStatus)
        |: (Json.Decode.field "inputs_satisfied" decodeBuildPrepStatus)
        |: (defaultTo Dict.empty <| Json.Decode.field "missing_input_reasons" <| Json.Decode.dict Json.Decode.string)


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
                    Err ("unknown build preparation status: " ++ unknown)



-- BuildResources


type alias BuildResources =
    { inputs : List BuildResourcesInput
    , outputs : List BuildResourcesOutput
    }


type alias BuildResourcesInput =
    { name : String
    , resource : String
    , type_ : String
    , version : Version
    , metadata : Metadata
    , firstOccurrence : Bool
    }


type alias BuildResourcesOutput =
    { resource : String
    , version : Version
    }


decodeBuildResources : Json.Decode.Decoder BuildResources
decodeBuildResources =
    Json.Decode.succeed BuildResources
        |: (Json.Decode.field "inputs" <| Json.Decode.list decodeResourcesInput)
        |: (Json.Decode.field "outputs" <| Json.Decode.list decodeResourcesOutput)


decodeResourcesInput : Json.Decode.Decoder BuildResourcesInput
decodeResourcesInput =
    Json.Decode.succeed BuildResourcesInput
        |: (Json.Decode.field "name" Json.Decode.string)
        |: (Json.Decode.field "resource" Json.Decode.string)
        |: (Json.Decode.field "type" Json.Decode.string)
        |: (Json.Decode.field "version" decodeVersion)
        |: (Json.Decode.field "metadata" decodeMetadata)
        |: (Json.Decode.field "first_occurrence" Json.Decode.bool)


decodeResourcesOutput : Json.Decode.Decoder BuildResourcesOutput
decodeResourcesOutput =
    Json.Decode.succeed BuildResourcesOutput
        |: (Json.Decode.field "resource" Json.Decode.string)
        |: (Json.Decode.field "version" <| Json.Decode.dict Json.Decode.string)



-- BuildPlan


type alias BuildPlan =
    { id : String
    , step : BuildStep
    }


type alias StepName =
    String


type BuildStep
    = BuildStepTask StepName
    | BuildStepGet StepName (Maybe Version)
    | BuildStepPut StepName
    | BuildStepDependentGet StepName
    | BuildStepAggregate (Array BuildPlan)
    | BuildStepDo (Array BuildPlan)
    | BuildStepOnSuccess HookedPlan
    | BuildStepOnFailure HookedPlan
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
        |: (Json.Decode.field "id" Json.Decode.string)
        |: Json.Decode.oneOf
            -- buckle up
            [ Json.Decode.field "task" <| lazy (\_ -> decodeBuildStepTask)
            , Json.Decode.field "get" <| lazy (\_ -> decodeBuildStepGet)
            , Json.Decode.field "put" <| lazy (\_ -> decodeBuildStepPut)
            , Json.Decode.field "dependent_get" <| lazy (\_ -> decodeBuildStepDependentGet)
            , Json.Decode.field "aggregate" <| lazy (\_ -> decodeBuildStepAggregate)
            , Json.Decode.field "do" <| lazy (\_ -> decodeBuildStepDo)
            , Json.Decode.field "on_success" <| lazy (\_ -> decodeBuildStepOnSuccess)
            , Json.Decode.field "on_failure" <| lazy (\_ -> decodeBuildStepOnFailure)
            , Json.Decode.field "ensure" <| lazy (\_ -> decodeBuildStepEnsure)
            , Json.Decode.field "try" <| lazy (\_ -> decodeBuildStepTry)
            , Json.Decode.field "retry" <| lazy (\_ -> decodeBuildStepRetry)
            , Json.Decode.field "timeout" <| lazy (\_ -> decodeBuildStepTimeout)
            ]


decodeBuildStepTask : Json.Decode.Decoder BuildStep
decodeBuildStepTask =
    Json.Decode.succeed BuildStepTask
        |: (Json.Decode.field "name" Json.Decode.string)


decodeBuildStepGet : Json.Decode.Decoder BuildStep
decodeBuildStepGet =
    Json.Decode.succeed BuildStepGet
        |: (Json.Decode.field "name" Json.Decode.string)
        |: (Json.Decode.maybe <| Json.Decode.field "version" decodeVersion)


decodeBuildStepPut : Json.Decode.Decoder BuildStep
decodeBuildStepPut =
    Json.Decode.succeed BuildStepPut
        |: (Json.Decode.field "name" Json.Decode.string)


decodeBuildStepDependentGet : Json.Decode.Decoder BuildStep
decodeBuildStepDependentGet =
    Json.Decode.succeed BuildStepDependentGet
        |: (Json.Decode.field "name" Json.Decode.string)


decodeBuildStepAggregate : Json.Decode.Decoder BuildStep
decodeBuildStepAggregate =
    Json.Decode.succeed BuildStepAggregate
        |: (Json.Decode.array (lazy (\_ -> decodeBuildPlan_)))


decodeBuildStepDo : Json.Decode.Decoder BuildStep
decodeBuildStepDo =
    Json.Decode.succeed BuildStepDo
        |: (Json.Decode.array (lazy (\_ -> decodeBuildPlan_)))


decodeBuildStepOnSuccess : Json.Decode.Decoder BuildStep
decodeBuildStepOnSuccess =
    Json.Decode.map BuildStepOnSuccess <|
        Json.Decode.succeed HookedPlan
            |: (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan_))
            |: (Json.Decode.field "on_success" <| lazy (\_ -> decodeBuildPlan_))


decodeBuildStepOnFailure : Json.Decode.Decoder BuildStep
decodeBuildStepOnFailure =
    Json.Decode.map BuildStepOnFailure <|
        Json.Decode.succeed HookedPlan
            |: (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan_))
            |: (Json.Decode.field "on_failure" <| lazy (\_ -> decodeBuildPlan_))


decodeBuildStepEnsure : Json.Decode.Decoder BuildStep
decodeBuildStepEnsure =
    Json.Decode.map BuildStepEnsure <|
        Json.Decode.succeed HookedPlan
            |: (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan_))
            |: (Json.Decode.field "ensure" <| lazy (\_ -> decodeBuildPlan_))


decodeBuildStepTry : Json.Decode.Decoder BuildStep
decodeBuildStepTry =
    Json.Decode.succeed BuildStepTry
        |: (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan_))


decodeBuildStepRetry : Json.Decode.Decoder BuildStep
decodeBuildStepRetry =
    Json.Decode.succeed BuildStepRetry
        |: (Json.Decode.array (lazy (\_ -> decodeBuildPlan_)))


decodeBuildStepTimeout : Json.Decode.Decoder BuildStep
decodeBuildStepTimeout =
    Json.Decode.succeed BuildStepTimeout
        |: (Json.Decode.field "step" <| lazy (\_ -> decodeBuildPlan_))



-- Info


type alias Info =
    { version : String
    }


decodeInfo : Json.Decode.Decoder Info
decodeInfo =
    Json.Decode.succeed Info
        |: (Json.Decode.field "version" Json.Decode.string)



-- Job


type alias JobName =
    String


type alias JobIdentifier =
    { teamName : TeamName
    , pipelineName : PipelineName
    , jobName : JobName
    }


type alias Job =
    { pipeline : PipelineIdentifier
    , name : JobName
    , url : String
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


decodeJob : PipelineIdentifier -> Json.Decode.Decoder Job
decodeJob pi =
    Json.Decode.succeed (Job pi)
        |: (Json.Decode.field "name" Json.Decode.string)
        |: (Json.Decode.field "url" Json.Decode.string)
        |: (Json.Decode.maybe (Json.Decode.field "next_build" decodeBuild))
        |: (Json.Decode.maybe (Json.Decode.field "finished_build" decodeBuild))
        |: (Json.Decode.maybe (Json.Decode.field "transition_build" decodeBuild))
        |: (defaultTo False <| Json.Decode.field "paused" Json.Decode.bool)
        |: (defaultTo False <| Json.Decode.field "disable_manual_trigger" Json.Decode.bool)
        |: (defaultTo [] <| Json.Decode.field "inputs" <| Json.Decode.list decodeJobInput)
        |: (defaultTo [] <| Json.Decode.field "outputs" <| Json.Decode.list decodeJobOutput)
        |: (defaultTo [] <| Json.Decode.field "groups" <| Json.Decode.list Json.Decode.string)


decodeJobInput : Json.Decode.Decoder JobInput
decodeJobInput =
    Json.Decode.succeed JobInput
        |: (Json.Decode.field "name" Json.Decode.string)
        |: (Json.Decode.field "resource" Json.Decode.string)
        |: (defaultTo [] <| Json.Decode.field "passed" <| Json.Decode.list Json.Decode.string)
        |: (defaultTo False <| Json.Decode.field "trigger" Json.Decode.bool)


decodeJobOutput : Json.Decode.Decoder JobOutput
decodeJobOutput =
    Json.Decode.succeed JobOutput
        |: (Json.Decode.field "name" Json.Decode.string)
        |: (Json.Decode.field "resource" Json.Decode.string)



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
    , url : String
    , paused : Bool
    , public : Bool
    , teamName : TeamName
    , groups : List PipelineGroup
    }


type alias PipelineGroup =
    { name : String
    , jobs : List String
    , resources : List String
    }


decodePipeline : Json.Decode.Decoder Pipeline
decodePipeline =
    Json.Decode.succeed Pipeline
        |: (Json.Decode.field "id" Json.Decode.int)
        |: (Json.Decode.field "name" Json.Decode.string)
        |: (Json.Decode.field "url" Json.Decode.string)
        |: (Json.Decode.field "paused" Json.Decode.bool)
        |: (Json.Decode.field "public" Json.Decode.bool)
        |: (Json.Decode.field "team_name" Json.Decode.string)
        |: (defaultTo [] <| Json.Decode.field "groups" (Json.Decode.list decodePipelineGroup))


decodePipelineGroup : Json.Decode.Decoder PipelineGroup
decodePipelineGroup =
    Json.Decode.succeed PipelineGroup
        |: (Json.Decode.field "name" Json.Decode.string)
        |: (defaultTo [] <| Json.Decode.field "jobs" <| Json.Decode.list Json.Decode.string)
        |: (defaultTo [] <| Json.Decode.field "resources" <| Json.Decode.list Json.Decode.string)



-- Resource


type alias Resource =
    { name : String
    , paused : Bool
    , failingToCheck : Bool
    , checkError : String
    }


type alias ResourceIdentifier =
    { teamName : String
    , pipelineName : String
    , resourceName : String
    }


type alias VersionedResource =
    { id : Int
    , version : Version
    , enabled : Bool
    , metadata : Metadata
    }


type alias VersionedResourceIdentifier =
    { teamName : String
    , pipelineName : String
    , resourceName : String
    , versionID : Int
    }


decodeResource : Json.Decode.Decoder Resource
decodeResource =
    Json.Decode.succeed Resource
        |: (Json.Decode.field "name" Json.Decode.string)
        |: (defaultTo False <| Json.Decode.field "paused" Json.Decode.bool)
        |: (defaultTo False <| Json.Decode.field "failing_to_check" Json.Decode.bool)
        |: (defaultTo "" <| Json.Decode.field "check_error" Json.Decode.string)


decodeVersionedResource : Json.Decode.Decoder VersionedResource
decodeVersionedResource =
    Json.Decode.succeed VersionedResource
        |: (Json.Decode.field "id" Json.Decode.int)
        |: (Json.Decode.field "version" decodeVersion)
        |: (Json.Decode.field "enabled" Json.Decode.bool)
        |: defaultTo [] (Json.Decode.field "metadata" decodeMetadata)



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
        |: (Json.Decode.field "name" Json.Decode.string)
        |: (Json.Decode.field "value" Json.Decode.string)



-- Team


type alias TeamName =
    String


type alias Team =
    { id : Int
    , name : TeamName
    }


decodeTeam : Json.Decode.Decoder Team
decodeTeam =
    Json.Decode.succeed Team
        |: (Json.Decode.field "id" Json.Decode.int)
        |: (Json.Decode.field "name" Json.Decode.string)



-- User


type alias User =
    { team : Team
    }


decodeUser : Json.Decode.Decoder User
decodeUser =
    Json.Decode.succeed User
        |: (Json.Decode.field "team" decodeTeam)



-- Helpers


dateFromSeconds : Float -> Date
dateFromSeconds =
    Date.fromTime << ((*) 1000)


lazy : (() -> Json.Decode.Decoder a) -> Json.Decode.Decoder a
lazy thunk =
    customDecoder Json.Decode.value
        (\js -> Json.Decode.decodeValue (thunk ()) js)


defaultTo : a -> Json.Decode.Decoder a -> Json.Decode.Decoder a
defaultTo default =
    Json.Decode.map (Maybe.withDefault default) << Json.Decode.maybe


customDecoder : Json.Decode.Decoder b -> (b -> Result String a) -> Json.Decode.Decoder a
customDecoder decoder toResult =
    Json.Decode.andThen
        (\a ->
            case toResult a of
                Ok b ->
                    Json.Decode.succeed b

                Err err ->
                    Json.Decode.fail err
        )
        decoder
