module Concourse exposing
  ( AuthMethod(..)
  , decodeAuthMethod

  , Build
  , BuildId
  , JobBuildIdentifier
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
  , JobIdentifier
  , JobInput
  , JobOutput
  , decodeJob

  , Pipeline
  , PipelineIdentifier
  , PipelineGroup
  , decodePipeline

  , Metadata
  , MetadataField
  , decodeMetadata

  , Team
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
    ( Json.Decode.object3
        (,,)
        ("type" := Json.Decode.string)
        (Json.Decode.maybe <| "display_name" := Json.Decode.string)
        (Json.Decode.maybe <| "auth_url" := Json.Decode.string)
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



-- Build


type alias BuildId =
  Int

type alias JobBuildIdentifier =
  { teamName : String
  , pipelineName : String
  , jobName : String
  , buildName : String
  }

type alias Build =
  { id : BuildId
  , url : String
  , name : String
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
  Json.Decode.object7 Build
    ("id" := Json.Decode.int)
    ("url" := Json.Decode.string)
    ("name" := Json.Decode.string)
    (Json.Decode.maybe (Json.Decode.object3 JobIdentifier
      ("job_name" := Json.Decode.string)
      ("team_name" := Json.Decode.string)
      ("pipeline_name" := Json.Decode.string)))
    ("status" := decodeBuildStatus)
    (Json.Decode.object2 BuildDuration
      (Json.Decode.maybe ("start_time" := (Json.Decode.map dateFromSeconds Json.Decode.float)))
      (Json.Decode.maybe ("end_time" := (Json.Decode.map dateFromSeconds Json.Decode.float))))
    (Json.Decode.maybe ("reap_time" := (Json.Decode.map dateFromSeconds Json.Decode.float)))

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
  Json.Decode.object2 BuildPlan ("id" := Json.Decode.string) <|
    Json.Decode.oneOf
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
  Json.Decode.object1 BuildStepTask ("name" := Json.Decode.string)

decodeBuildStepGet : Json.Decode.Decoder BuildStep
decodeBuildStepGet =
  Json.Decode.object2 BuildStepGet
    ("name" := Json.Decode.string)
    (Json.Decode.maybe <| "version" := decodeVersion)

decodeBuildStepPut : Json.Decode.Decoder BuildStep
decodeBuildStepPut =
  Json.Decode.object1 BuildStepPut ("name" := Json.Decode.string)

decodeBuildStepDependentGet : Json.Decode.Decoder BuildStep
decodeBuildStepDependentGet =
  Json.Decode.object1 BuildStepDependentGet ("name" := Json.Decode.string)

decodeBuildStepAggregate : Json.Decode.Decoder BuildStep
decodeBuildStepAggregate =
  Json.Decode.object1 BuildStepAggregate (Json.Decode.array (lazy (\_ -> decodeBuildPlan')))

decodeBuildStepDo : Json.Decode.Decoder BuildStep
decodeBuildStepDo =
  Json.Decode.object1 BuildStepDo (Json.Decode.array (lazy (\_ -> decodeBuildPlan')))

decodeBuildStepOnSuccess : Json.Decode.Decoder BuildStep
decodeBuildStepOnSuccess =
  Json.Decode.map BuildStepOnSuccess <|
    Json.Decode.object2 HookedPlan ("step" := lazy (\_ -> decodeBuildPlan')) ("on_success" := lazy (\_ -> decodeBuildPlan'))

decodeBuildStepOnFailure : Json.Decode.Decoder BuildStep
decodeBuildStepOnFailure =
  Json.Decode.map BuildStepOnFailure <|
    Json.Decode.object2 HookedPlan ("step" := lazy (\_ -> decodeBuildPlan')) ("on_failure" := lazy (\_ -> decodeBuildPlan'))

decodeBuildStepEnsure : Json.Decode.Decoder BuildStep
decodeBuildStepEnsure =
  Json.Decode.map BuildStepEnsure <|
    Json.Decode.object2 HookedPlan ("step" := lazy (\_ -> decodeBuildPlan')) ("ensure" := lazy (\_ -> decodeBuildPlan'))

decodeBuildStepTry : Json.Decode.Decoder BuildStep
decodeBuildStepTry =
  Json.Decode.object1 BuildStepTry ("step" := lazy (\_ -> decodeBuildPlan'))

decodeBuildStepRetry : Json.Decode.Decoder BuildStep
decodeBuildStepRetry =
  Json.Decode.object1 BuildStepRetry (Json.Decode.array (lazy (\_ -> decodeBuildPlan')))

decodeBuildStepTimeout : Json.Decode.Decoder BuildStep
decodeBuildStepTimeout =
  Json.Decode.object1 BuildStepTimeout ("step" := lazy (\_ -> decodeBuildPlan'))



-- Job


type alias JobIdentifier =
  { teamName : String
  , pipelineName : String
  , jobName : String
  }

type alias Job =
  { teamName : String
  , pipelineName : String
  , name : String
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

decodeJob : String -> String -> Json.Decode.Decoder Job
decodeJob teamName pipelineName =
  Json.Decode.succeed (Job teamName pipelineName)
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


type alias PipelineIdentifier =
  { teamName : String
  , pipelineName : String
  }

type alias Pipeline =
  { name : String
  , url : String
  , paused : Bool
  , public : Bool
  , teamName : String
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

type alias Team =
  { id : Int
  , name : String
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
