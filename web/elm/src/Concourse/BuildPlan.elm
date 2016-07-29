module Concourse.BuildPlan exposing (..)

import Array exposing (Array)
import Http
import Task exposing (Task)
import Json.Decode exposing ((:=))

import Concourse.Version exposing (Version)

type alias BuildPlan =
  { id : String
  , step : BuildStep
  }

type alias StepName =
  String

type BuildStep
  = Task StepName
  | Get StepName (Maybe Version)
  | Put StepName
  | DependentGet StepName
  | Aggregate (Array BuildPlan)
  | Do (Array BuildPlan)
  | OnSuccess HookedPlan
  | OnFailure HookedPlan
  | Ensure HookedPlan
  | Try BuildPlan
  | Retry (Array BuildPlan)
  | Timeout BuildPlan

type alias HookedPlan =
  { step : BuildPlan
  , hook : BuildPlan
  }

type alias BuildId =
  Int

fetch : BuildId -> Task Http.Error BuildPlan
fetch buildId =
  Http.get decode ("/api/v1/builds/" ++ toString buildId ++ "/plan")

decode : Json.Decode.Decoder BuildPlan
decode =
  Json.Decode.at ["plan"] <|
    decodePlan

decodePlan : Json.Decode.Decoder BuildPlan
decodePlan =
  Json.Decode.object2 BuildPlan ("id" := Json.Decode.string) <|
    Json.Decode.oneOf
      [ "task" := lazy (\_ -> decodeTask)
      , "get" := lazy (\_ -> decodeGet)
      , "put" := lazy (\_ -> decodePut)
      , "dependent_get" := lazy (\_ -> decodeDependentGet)
      , "aggregate" := lazy (\_ -> decodeAggregate)
      , "do" := lazy (\_ -> decodeDo)
      , "on_success" := lazy (\_ -> decodeOnSuccess)
      , "on_failure" := lazy (\_ -> decodeOnFailure)
      , "ensure" := lazy (\_ -> decodeEnsure)
      , "try" := lazy (\_ -> decodeTry)
      , "retry" := lazy (\_ -> decodeRetry)
      , "timeout" := lazy (\_ -> decodeTimeout)
      ]

decodeTask : Json.Decode.Decoder BuildStep
decodeTask =
  Json.Decode.object1 Task ("name" := Json.Decode.string)

decodeGet : Json.Decode.Decoder BuildStep
decodeGet =
  Json.Decode.object2 Get
    ("name" := Json.Decode.string)
    (Json.Decode.maybe <|
      "version" := Concourse.Version.decode)

decodePut : Json.Decode.Decoder BuildStep
decodePut =
  Json.Decode.object1 Put ("name" := Json.Decode.string)

decodeDependentGet : Json.Decode.Decoder BuildStep
decodeDependentGet =
  Json.Decode.object1 DependentGet ("name" := Json.Decode.string)

decodeAggregate : Json.Decode.Decoder BuildStep
decodeAggregate =
  Json.Decode.object1 Aggregate (Json.Decode.array (lazy (\_ -> decodePlan)))

decodeDo : Json.Decode.Decoder BuildStep
decodeDo =
  Json.Decode.object1 Do (Json.Decode.array (lazy (\_ -> decodePlan)))

decodeOnSuccess : Json.Decode.Decoder BuildStep
decodeOnSuccess =
  Json.Decode.map OnSuccess <|
    Json.Decode.object2 HookedPlan ("step" := lazy (\_ -> decodePlan)) ("on_success" := lazy (\_ -> decodePlan))

decodeOnFailure : Json.Decode.Decoder BuildStep
decodeOnFailure =
  Json.Decode.map OnFailure <|
    Json.Decode.object2 HookedPlan ("step" := lazy (\_ -> decodePlan)) ("on_failure" := lazy (\_ -> decodePlan))

decodeEnsure : Json.Decode.Decoder BuildStep
decodeEnsure =
  Json.Decode.map Ensure <|
    Json.Decode.object2 HookedPlan ("step" := lazy (\_ -> decodePlan)) ("ensure" := lazy (\_ -> decodePlan))

decodeTry : Json.Decode.Decoder BuildStep
decodeTry =
  Json.Decode.object1 Try ("step" := lazy (\_ -> decodePlan))

decodeRetry : Json.Decode.Decoder BuildStep
decodeRetry =
  Json.Decode.object1 Retry (Json.Decode.array (lazy (\_ -> decodePlan)))

decodeTimeout : Json.Decode.Decoder BuildStep
decodeTimeout =
  Json.Decode.object1 Timeout ("step" := lazy (\_ -> decodePlan))

lazy : (() -> Json.Decode.Decoder a) -> Json.Decode.Decoder a
lazy thunk =
  Json.Decode.customDecoder Json.Decode.value
      (\js -> Json.Decode.decodeValue (thunk ()) js)
