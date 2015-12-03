module BuildPlan where

import Array exposing (Array)
import Dict exposing (Dict)
import Time exposing (Time)
import Json.Decode exposing ((:=))

type alias BuildPlan =
  { id : String
  , step : BuildStep
  }

type alias StepName = String

type alias Version =
  Dict String String

type BuildStep
  = Task StepName
  | Get StepName (Maybe Version)
  | Put StepName
  | DependentGet StepName
  | Aggregate (Array BuildPlan)
  | OnSuccess HookedPlan
  | OnFailure HookedPlan
  | Ensure HookedPlan
  | Try BuildPlan
  | Timeout BuildPlan

type alias HookedPlan =
  { step : BuildPlan
  , hook : BuildPlan
  }

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
      , "on_success" := lazy (\_ -> decodeOnSuccess)
      , "on_failure" := lazy (\_ -> decodeOnFailure)
      , "ensure" := lazy (\_ -> decodeEnsure)
      , "try" := lazy (\_ -> decodeTry)
      , "timeout" := lazy (\_ -> decodeTimeout)
      ]

decodeTask =
  Json.Decode.object1 Task ("name" := Json.Decode.string)

decodeGet =
  Json.Decode.object2 Get
    ("name" := Json.Decode.string)
    (Json.Decode.maybe <| "version" := (Json.Decode.dict Json.Decode.string))

decodePut =
  Json.Decode.object1 Put ("name" := Json.Decode.string)

decodeDependentGet =
  Json.Decode.object1 DependentGet ("name" := Json.Decode.string)

decodeAggregate =
  Json.Decode.object1 Aggregate (Json.Decode.array (lazy (\_ -> decodePlan)))

decodeOnSuccess =
  Json.Decode.map OnSuccess <|
    Json.Decode.object2 HookedPlan ("step" := lazy (\_ -> decodePlan)) ("on_success" := lazy (\_ -> decodePlan))

decodeOnFailure =
  Json.Decode.map OnFailure <|
    Json.Decode.object2 HookedPlan ("step" := lazy (\_ -> decodePlan)) ("on_failure" := lazy (\_ -> decodePlan))

decodeEnsure =
  Json.Decode.map Ensure <|
    Json.Decode.object2 HookedPlan ("step" := lazy (\_ -> decodePlan)) ("ensure" := lazy (\_ -> decodePlan))

decodeTry =
  Json.Decode.object1 Try ("step" := lazy (\_ -> decodePlan))

decodeTimeout =
  Json.Decode.object1 Timeout ("step" := lazy (\_ -> decodePlan))

lazy : (() -> Json.Decode.Decoder a) -> Json.Decode.Decoder a
lazy thunk =
  Json.Decode.customDecoder Json.Decode.value
      (\js -> Json.Decode.decodeValue (thunk ()) js)
