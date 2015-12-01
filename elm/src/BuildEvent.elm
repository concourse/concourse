module BuildEvent where

import Date exposing (Date)
import Json.Decode as Json exposing ((:=))

type BuildEvent
  = BuildStatus BuildStatus Date
  | FinishGet Origin Int
  | FinishPut Origin Int
  | InitializeTask Origin
  | StartTask Origin
  | FinishTask Origin Int
  | Log Origin String
  | Error Origin String

type alias BuildEventEnvelope =
  { event : String
  , version : String
  , value : Json.Value
  }

type alias Origin =
  { stepName : String
  , stepType : StepType
  , source : String
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
      Json.decodeValue (Json.object2 Error ("origin" := decodeOrigin) ("message" := Json.string)) e.value

    "initialize-task" ->
      Json.decodeValue (Json.object1 InitializeTask ("origin" := decodeOrigin)) e.value

    "start-task" ->
      Json.decodeValue (Json.object1 StartTask ("origin" := decodeOrigin)) e.value

    "finish-task" ->
      Json.decodeValue (Json.object2 FinishTask ("origin" := decodeOrigin) ("exit_status" := Json.int)) e.value

    "finish-get" ->
      Json.decodeValue (Json.object2 FinishGet ("origin" := decodeOrigin) ("exit_status" := Json.int)) e.value

    "finish-put" ->
      Json.decodeValue (Json.object2 FinishPut ("origin" := decodeOrigin) ("exit_status" := Json.int)) e.value

    unknown ->
      Err ("unknown event type: " ++ unknown)

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
  Json.object4 Origin
    ("name" := Json.string)
    decodeStepType
    ("source" := Json.string)
    ("id" := Json.string)

decodeStepType : Json.Decoder StepType
decodeStepType =
  Json.customDecoder ("type" := Json.string) <| \t ->
    case t of
      "task" -> Ok StepTypeTask
      "get" -> Ok StepTypeGet
      "put" -> Ok StepTypePut
      unknown -> Err ("unknown step type: " ++ unknown)
