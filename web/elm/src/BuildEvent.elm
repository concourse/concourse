module BuildEvent where

import Date exposing (Date)
import Dict exposing (Dict)
import Json.Decode as Json exposing ((:=))

type BuildEvent
  = BuildStatus BuildStatus Date
  | FinishGet Origin Int Version (List MetadataField)
  | FinishPut Origin Int Version (List MetadataField)
  | InitializeTask Origin
  | StartTask Origin
  | FinishTask Origin Int
  | Log Origin String
  | Error Origin String
  | BuildError String

type alias BuildEventEnvelope =
  { event : String
  , version : String
  , value : Json.Value
  }

type alias Origin =
  { source : String
  , id : String
  }

type StepType
  = StepTypeTask
  | StepTypeGet
  | StepTypePut

type BuildStatus
  = BuildStatusPending
  | BuildStatusStarted
  | BuildStatusSucceeded
  | BuildStatusFailed
  | BuildStatusErrored
  | BuildStatusAborted

type alias Version =
  Dict String String

type alias Metadata =
  List MetadataField

type alias MetadataField =
  { name : String
  , value : String
  }

decode : Json.Decoder BuildEvent
decode = Json.customDecoder decodeEnvelope decodeEvent

decodeEnvelope : Json.Decoder BuildEventEnvelope
decodeEnvelope =
  Json.object3 BuildEventEnvelope
    ("event" := Json.string)
    ("version" := Json.string)
    ("data" := Json.value)

dateFromSeconds : Float -> Date
dateFromSeconds = Date.fromTime << ((*) 1000)

decodeEvent : BuildEventEnvelope -> Result String BuildEvent
decodeEvent e =
  case e.event of
    "status" ->
      Json.decodeValue (Json.object2 BuildStatus decodeStatus ("time" := Json.map dateFromSeconds Json.float)) e.value

    "log" ->
      Json.decodeValue (Json.object2 Log ("origin" := decodeOrigin) ("payload" := Json.string)) e.value

    "error" ->
      Json.decodeValue decodeErrorEvent e.value

    "initialize-task" ->
      Json.decodeValue (Json.object1 InitializeTask ("origin" := decodeOrigin)) e.value

    "start-task" ->
      Json.decodeValue (Json.object1 StartTask ("origin" := decodeOrigin)) e.value

    "finish-task" ->
      Json.decodeValue (Json.object2 FinishTask ("origin" := decodeOrigin) ("exit_status" := Json.int)) e.value

    "finish-get" ->
      Json.decodeValue (decodeFinishResource FinishGet) e.value

    "finish-put" ->
      Json.decodeValue (decodeFinishResource FinishPut) e.value

    unknown ->
      Err ("unknown event type: " ++ unknown)

decodeFinishResource cons =
  Json.object4 cons
    ("origin" := decodeOrigin)
    ("exit_status" := Json.int)
    ("version" := decodeVersion)
    ("metadata" := decodeMetadata)

decodeVersion =
  Json.dict Json.string

decodeMetadata =
  Json.list decodeMetadataField

decodeMetadataField =
  Json.object2 MetadataField
    ("name" := Json.string)
    ("value" := Json.string)

decodeErrorEvent : Json.Decoder BuildEvent
decodeErrorEvent =
  Json.oneOf
    [ Json.object2 Error ("origin" := decodeOrigin) ("message" := Json.string)
    , Json.object1 BuildError ("message" := Json.string)
    ]

decodeStatus : Json.Decoder BuildStatus
decodeStatus =
  Json.customDecoder ("status" := Json.string) <| \status ->
   case status of
      "started" -> Ok BuildStatusStarted
      "succeeded" -> Ok BuildStatusSucceeded
      "failed" -> Ok BuildStatusFailed
      "errored" -> Ok BuildStatusErrored
      "aborted" -> Ok BuildStatusAborted
      unknown -> Err ("unknown build status: " ++ unknown)

decodeOrigin : Json.Decoder Origin
decodeOrigin =
  Json.object2 Origin
    (Json.map (Maybe.withDefault "") << Json.maybe <| "source" := Json.string)
    ("id" := Json.string)

decodeStepType : Json.Decoder StepType
decodeStepType =
  Json.customDecoder ("type" := Json.string) <| \t ->
    case t of
      "task" -> Ok StepTypeTask
      "get" -> Ok StepTypeGet
      "put" -> Ok StepTypePut
      unknown -> Err ("unknown step type: " ++ unknown)
