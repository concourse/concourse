module Concourse exposing
  ( AuthMethod(..)
  , decodeAuthMethod

  , AuthToken
  , decodeAuthToken

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
import Json.Decode exposing ((:=))
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
  Json.Decode.customDecoder
    ( Json.Decode.succeed (,,)
        |: ("type" := Json.Decode.string)
        |: (Json.Decode.maybe <| "display_name" := Json.Decode.string)
        |: (Json.Decode.maybe <| "auth_url" := Json.Decode.string)
    )
    authMethodFromTuple

authMethodFromTuple : (String, Maybe String, Maybe String) -> Result String AuthMethod
authMethodFromTuple tuple =
  case tuple of
    ("basic", _, _) ->
      Ok AuthMethodBasic
    ("oauth", Just displayName, Just authUrl) ->
      Ok (AuthMethodOAuth { displayName = displayName, authUrl = authUrl })
    ("oauth", _, _) ->
      Err "missing fields in oauth auth method"
    _ ->
      Err "unknown value for auth method type"


-- AuthToken

type alias AuthToken =
  String

decodeAuthToken : Json.Decode.Decoder AuthToken
decodeAuthToken =
  Json.Decode.customDecoder
    ( Json.Decode.succeed (,)
        |: ("type" := Json.Decode.string)
        |: ("value" := Json.Decode.string)
    )
    authTokenFromTuple

authTokenFromTuple : (String, String) -> Result String AuthToken
authTokenFromTuple (t, token) =
  case t of
    "Bearer" ->
      Ok token
    _ ->
      Err "unknown token type"


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
    |: ("id" := Json.Decode.int)
    |: ("url" := Json.Decode.string)
    |: ("name" := Json.Decode.string)
    |: (Json.Decode.maybe (Json.Decode.succeed JobIdentifier
      |: ("team_name" := Json.Decode.string)
      |: ("pipeline_name" := Json.Decode.string)
      |: ("job_name" := Json.Decode.string)))
    |: ("status" := decodeBuildStatus)
    |: (Json.Decode.succeed BuildDuration
      |: (Json.Decode.maybe ("start_time" := (Json.Decode.map dateFromSeconds Json.Decode.float)))
      |: (Json.Decode.maybe ("end_time" := (Json.Decode.map dateFromSeconds Json.Decode.float))))
    |: (Json.Decode.maybe ("reap_time" := (Json.Decode.map dateFromSeconds Json.Decode.float)))

decodeBuildStatus : Json.Decode.Decoder BuildStatus
decodeBuildStatus =
  Json.Decode.customDecoder Json.Decode.string <| \status ->
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
    |: ("paused_pipeline" := decodeBuildPrepStatus)
    |: ("paused_job" := decodeBuildPrepStatus)
    |: ("max_running_builds" := decodeBuildPrepStatus)
    |: ("inputs" := Json.Decode.dict decodeBuildPrepStatus)
    |: ("inputs_satisfied" := decodeBuildPrepStatus)
    |: (defaultTo Dict.empty <| "missing_input_reasons" := Json.Decode.dict Json.Decode.string)

decodeBuildPrepStatus : Json.Decode.Decoder BuildPrepStatus
decodeBuildPrepStatus =
  Json.Decode.customDecoder Json.Decode.string <| \status ->
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
  , type' : String
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
    |: ("inputs" := Json.Decode.list decodeResourcesInput)
    |: ("outputs" := Json.Decode.list decodeResourcesOutput)

decodeResourcesInput : Json.Decode.Decoder BuildResourcesInput
decodeResourcesInput =
  Json.Decode.succeed BuildResourcesInput
    |: ("name" := Json.Decode.string)
    |: ("resource" := Json.Decode.string)
    |: ("type" := Json.Decode.string)
    |: ("version" := decodeVersion)
    |: ("metadata" := decodeMetadata)
    |: ("first_occurrence" := Json.Decode.bool)

decodeResourcesOutput : Json.Decode.Decoder BuildResourcesOutput
decodeResourcesOutput =
  Json.Decode.succeed BuildResourcesOutput
    |: ("resource" := Json.Decode.string)
    |: ("version" := Json.Decode.dict Json.Decode.string)



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
  Json.Decode.at ["plan"] <|
    decodeBuildPlan'

decodeBuildPlan' : Json.Decode.Decoder BuildPlan
decodeBuildPlan' =
  Json.Decode.succeed BuildPlan
    |: ("id" := Json.Decode.string)
    |: Json.Decode.oneOf
        -- buckle up
        [ "task" := lazy (\_ -> decodeBuildStepTask)
        , "get" := lazy (\_ -> decodeBuildStepGet)
        , "put" := lazy (\_ -> decodeBuildStepPut)
        , "dependent_get" := lazy (\_ -> decodeBuildStepDependentGet)
        , "aggregate" := lazy (\_ -> decodeBuildStepAggregate)
        , "do" := lazy (\_ -> decodeBuildStepDo)
        , "on_success" := lazy (\_ -> decodeBuildStepOnSuccess)
        , "on_failure" := lazy (\_ -> decodeBuildStepOnFailure)
        , "ensure" := lazy (\_ -> decodeBuildStepEnsure)
        , "try" := lazy (\_ -> decodeBuildStepTry)
        , "retry" := lazy (\_ -> decodeBuildStepRetry)
        , "timeout" := lazy (\_ -> decodeBuildStepTimeout)
        ]

decodeBuildStepTask : Json.Decode.Decoder BuildStep
decodeBuildStepTask =
  Json.Decode.succeed BuildStepTask
    |: ("name" := Json.Decode.string)

decodeBuildStepGet : Json.Decode.Decoder BuildStep
decodeBuildStepGet =
  Json.Decode.succeed BuildStepGet
    |: ("name" := Json.Decode.string)
    |: (Json.Decode.maybe <| "version" := decodeVersion)

decodeBuildStepPut : Json.Decode.Decoder BuildStep
decodeBuildStepPut =
  Json.Decode.succeed BuildStepPut
    |: ("name" := Json.Decode.string)

decodeBuildStepDependentGet : Json.Decode.Decoder BuildStep
decodeBuildStepDependentGet =
  Json.Decode.succeed BuildStepDependentGet
    |: ("name" := Json.Decode.string)

decodeBuildStepAggregate : Json.Decode.Decoder BuildStep
decodeBuildStepAggregate =
  Json.Decode.succeed BuildStepAggregate
    |: (Json.Decode.array (lazy (\_ -> decodeBuildPlan')))

decodeBuildStepDo : Json.Decode.Decoder BuildStep
decodeBuildStepDo =
  Json.Decode.succeed BuildStepDo
    |: (Json.Decode.array (lazy (\_ -> decodeBuildPlan')))

decodeBuildStepOnSuccess : Json.Decode.Decoder BuildStep
decodeBuildStepOnSuccess =
  Json.Decode.map BuildStepOnSuccess <|
    Json.Decode.succeed HookedPlan
      |: ("step" := lazy (\_ -> decodeBuildPlan'))
      |: ("on_success" := lazy (\_ -> decodeBuildPlan'))

decodeBuildStepOnFailure : Json.Decode.Decoder BuildStep
decodeBuildStepOnFailure =
  Json.Decode.map BuildStepOnFailure <|
    Json.Decode.succeed HookedPlan
      |: ("step" := lazy (\_ -> decodeBuildPlan'))
      |: ("on_failure" := lazy (\_ -> decodeBuildPlan'))

decodeBuildStepEnsure : Json.Decode.Decoder BuildStep
decodeBuildStepEnsure =
  Json.Decode.map BuildStepEnsure <|
    Json.Decode.succeed HookedPlan
      |: ("step" := lazy (\_ -> decodeBuildPlan'))
      |: ("ensure" := lazy (\_ -> decodeBuildPlan'))

decodeBuildStepTry : Json.Decode.Decoder BuildStep
decodeBuildStepTry =
  Json.Decode.succeed BuildStepTry
    |: ("step" := lazy (\_ -> decodeBuildPlan'))

decodeBuildStepRetry : Json.Decode.Decoder BuildStep
decodeBuildStepRetry =
  Json.Decode.succeed BuildStepRetry
    |: (Json.Decode.array (lazy (\_ -> decodeBuildPlan')))

decodeBuildStepTimeout : Json.Decode.Decoder BuildStep
decodeBuildStepTimeout =
  Json.Decode.succeed BuildStepTimeout
    |: ("step" := lazy (\_ -> decodeBuildPlan'))



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
    |: ("name" := Json.Decode.string)
    |: ("url" := Json.Decode.string)
    |: (Json.Decode.maybe ("next_build" := decodeBuild))
    |: (Json.Decode.maybe ("finished_build" := decodeBuild))
    |: (defaultTo False <| "paused" := Json.Decode.bool)
    |: (defaultTo False <| "disable_manual_trigger" := Json.Decode.bool)
    |: (defaultTo [] <| "inputs" := Json.Decode.list decodeJobInput)
    |: (defaultTo [] <| "outputs" := Json.Decode.list decodeJobOutput)
    |: (defaultTo [] <| "groups" := Json.Decode.list Json.Decode.string)

decodeJobInput : Json.Decode.Decoder JobInput
decodeJobInput =
  Json.Decode.succeed JobInput
    |: ("name" := Json.Decode.string)
    |: ("resource" := Json.Decode.string)
    |: (defaultTo [] <| "passed" := Json.Decode.list Json.Decode.string)
    |: (defaultTo False <| "trigger" := Json.Decode.bool)

decodeJobOutput : Json.Decode.Decoder JobOutput
decodeJobOutput =
  Json.Decode.succeed JobOutput
    |: ("name" := Json.Decode.string)
    |: ("resource" := Json.Decode.string)



-- Pipeline


type alias PipelineName =
  String

type alias PipelineIdentifier =
  { teamName : TeamName
  , pipelineName : PipelineName
  }

type alias Pipeline =
  { name : PipelineName
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
    |: ("name" := Json.Decode.string)
    |: ("url" := Json.Decode.string)
    |: ("paused" := Json.Decode.bool)
    |: ("public" := Json.Decode.bool)
    |: ("team_name" := Json.Decode.string)
    |: (defaultTo [] <| "groups" := (Json.Decode.list decodePipelineGroup))

decodePipelineGroup : Json.Decode.Decoder PipelineGroup
decodePipelineGroup =
  Json.Decode.succeed PipelineGroup
    |: ("name" := Json.Decode.string)
    |: (defaultTo [] <| "jobs" := Json.Decode.list Json.Decode.string)
    |: (defaultTo [] <| "resources" := Json.Decode.list Json.Decode.string)


-- Resource

type alias Resource =
  { name : String
  , paused: Bool
  , failingToCheck: Bool
  , checkError: String
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
    |: ("name" := Json.Decode.string)
    |: (defaultTo False <| "paused" := Json.Decode.bool)
    |: (defaultTo False <| "failing_to_check" := Json.Decode.bool)
    |: (defaultTo "" <| "check_error" := Json.Decode.string)

decodeVersionedResource : Json.Decode.Decoder VersionedResource
decodeVersionedResource =
  Json.Decode.succeed VersionedResource
    |: ("id" := Json.Decode.int)
    |: ("version" := decodeVersion)
    |: ("enabled" := Json.Decode.bool)
    |: defaultTo [] ("metadata" := decodeMetadata)


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
    |: ("name" := Json.Decode.string)
    |: ("value" := Json.Decode.string)



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
    |: ("id" := Json.Decode.int)
    |: ("name" := Json.Decode.string)



-- User


type alias User =
  { team : Team
  }

decodeUser : Json.Decode.Decoder User
decodeUser =
  Json.Decode.succeed User
    |: ("team" := decodeTeam)



-- Helpers


dateFromSeconds : Float -> Date
dateFromSeconds =
  Date.fromTime << ((*) 1000)

lazy : (() -> Json.Decode.Decoder a) -> Json.Decode.Decoder a
lazy thunk =
  Json.Decode.customDecoder Json.Decode.value
      (\js -> Json.Decode.decodeValue (thunk ()) js)

defaultTo : a -> Json.Decode.Decoder a -> Json.Decode.Decoder a
defaultTo default =
  Json.Decode.map (Maybe.withDefault default) << Json.Decode.maybe
